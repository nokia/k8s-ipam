/*
Copyright 2022 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ipam

/*
// +k8s:deepcopy-gen=false
type Allocation struct {
	Origin          ipamv1alpha1.Origin        `json:"origin,omitempty" yaml:"origin,omitempty"`
	NamespacedName  types.NamespacedName       `json:"namespacedName,omitempty" yaml:"namespacedName,omitempty"`
	PrefixKind      ipamv1alpha1.PrefixKind    `json:"prefixKind,omitempty" yaml:"prefixKind,omitempty"`
	NetworkInstance string                     `json:"networkInstance,omitempty" yaml:"networkInstance,omitempty"`
	Prefix          string                     `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	AddresFamily    ipamv1alpha1.AddressFamily `json:"addressFamily,omitempty" yaml:"addressFamily,omitempty"` // used for alloc w/o prefix
	PrefixLength    uint8                      `json:"prefixLength,omitempty" yaml:"prefixLength,omitempty"`   // used for alloc w/o prefix and prefix kind = pool
	SpecLabels      map[string]string          `json:"specLabels,omitempty" yaml:"specLabels,omitempty"`       // labels in the spec
	SelectorLabels  map[string]string          `json:"selectorLabels,omitempty" yaml:"selectorLabels,omitempty"`
	//SubnetName      string                     `json:"subnetName,omitempty" yaml:"subnetName,omitempty"`     // explicitly mentioned for prefixkind network
	//specificLabels  map[string]string
}

type AllocatedPrefix struct {
	Prefix  string
	Gateway string
}
func (r *Allocation) GetName() string {
	return r.NamespacedName.Name
}

func (r *Allocation) GetNameSpace() string {
	return r.NamespacedName.Namespace
}

func (r *Allocation) GetOrigin() ipamv1alpha1.Origin {
	return r.Origin
}

func (r *Allocation) GetNetworkInstance() string {
	return r.NetworkInstance
}

func (r *Allocation) GetNINamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: r.NamespacedName.Namespace,
		Name:      r.NetworkInstance,
	}
}

func (r *Allocation) GetPrefixKind() ipamv1alpha1.PrefixKind {
	return r.PrefixKind
}

func (r *Allocation) GetAddressFamily() ipamv1alpha1.AddressFamily {
	return r.AddresFamily
}

func (r *Allocation) GetPrefix() string {
	return r.Prefix
}

func (r *Allocation) GetIPPrefix() netaddr.IPPrefix {
	return netaddr.MustParseIPPrefix(r.Prefix)
}

//func (r *Allocation) GetSubnetName() string {
//	return r.SubnetName
//}

func (r *Allocation) GetLabels() map[string]string {
	l := map[string]string{}
	for k, v := range r.SpecLabels {
		l[k] = v
	}
	return l
}

func (r *Allocation) GetSelectorLabels() map[string]string {
	l := map[string]string{}
	for k, v := range r.SelectorLabels {
		l[k] = v
	}
	return l
}

func (r *Allocation) GetGatewayLabelSelector() (labels.Selector, error) {
	l := map[string]string{
		ipamv1alpha1.NephioGatewayKey: "true",
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		// exclude any key that is not network and networkinstance
		if k == ipamv1alpha1.NephioSubnetKey ||
			//k == ipamv1alpha1.NephioNetworkInstanceKey ||
			k == ipamv1alpha1.NephioGatewayKey {
			req, err := labels.NewRequirement(k, selection.In, []string{v})
			if err != nil {
				return nil, err
			}
			fullselector = fullselector.Add(*req)
		}
	}
	for k, v := range r.GetSelectorLabels() {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *Allocation) GetSubnetLabelSelector() (labels.Selector, error) {
	//p := netaddr.MustParseIPPrefix(r.Prefix)
	//af := iputil.GetAddressFamily(p)

	l := map[string]string{
		ipamv1alpha1.NephioSubnetKey: iputil.GetSubnetName(r.Prefix),
		//ipamv1alpha1.NephioSubnetNameKey:    r.SubnetName,
		//ipamv1alpha1.NephioAddressFamilyKey: string(af),
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *Allocation) GetLabelSelector() (labels.Selector, error) {
	l := r.GetSelectorLabels()
	// For prefixkind network we want to allocate only prefixes within a network
	if r.PrefixKind == ipamv1alpha1.PrefixKindNetwork {
		l[ipamv1alpha1.NephioPrefixKindKey] = string(ipamv1alpha1.PrefixKindNetwork)
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *Allocation) GetAllocSelector() (labels.Selector, error) {
	l := map[string]string{
		ipamv1alpha1.NephioIPAllocactionNameKey: r.GetName(),
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *Allocation) GetFullSelector() (labels.Selector, error) {
	fullselector := labels.NewSelector()
	for k, v := range r.GetLabels() {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	for k, v := range r.GetSelectorLabels() {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *Allocation) GetFullLabels() map[string]string {
	l := make(map[string]string)
	for k, v := range r.GetLabels() {
		l[k] = v
	}
	for k, v := range r.GetSelectorLabels() {
		l[k] = v
	}
	return l
}

func (r *Allocation) GetPrefixFromNewAlloc() string {
	p := r.Prefix
	parentPrefixLength, ok := r.GetLabels()[ipamv1alpha1.NephioParentPrefixLengthKey]
	if ok {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return p
}

func (r *Allocation) GetPrefixLengthFromRoute(route *table.Route) uint8 {
	if r.PrefixLength != 0 {
		return r.PrefixLength
	}
	if route.IPPrefix().IP().Is4() {
		return 32
	}
	return 128
}
*/

/*
func BuildAllocationFromIPPrefix(cr *ipamv1alpha1.IPPrefix) *Allocation {
	p := netaddr.MustParseIPPrefix(cr.Spec.Prefix)
	return &Allocation{
		NamespacedName: types.NamespacedName{
			Name:      cr.GetName(),
			Namespace: cr.GetNamespace(),
		},
		Origin:          ipamv1alpha1.OriginIPPrefix,
		NetworkInstance: cr.Spec.NetworkInstance,
		PrefixKind:      cr.Spec.PrefixKind,
		AddresFamily:    iputil.GetAddressFamily(p),
		Prefix:          cr.Spec.Prefix,
		PrefixLength:    uint8(iputil.GetPrefixLengthAsInt(p)),
		//SubnetName:      cr.Spec.SubnetName,
		SpecLabels: cr.Spec.Labels,
	}
}

func BuildAllocationFromNetworkInstancePrefix(cr *ipamv1alpha1.NetworkInstance, prefix *ipamv1alpha1.Prefix) *Allocation {
	p := netaddr.MustParseIPPrefix(prefix.Prefix)
	return &Allocation{
		NamespacedName: types.NamespacedName{
			Name:      GetNameFromNetworkInstancePrefix(cr, prefix.Prefix), // name fro aggregate derived from prefix and crName
			Namespace: cr.GetNamespace(),
		},
		Origin:          ipamv1alpha1.OriginNetworkInstance,
		NetworkInstance: cr.GetName(),
		PrefixKind:      ipamv1alpha1.PrefixKindAggregate, // always used as an aggregate
		AddresFamily:    iputil.GetAddressFamily(p),
		Prefix:          prefix.Prefix,
		PrefixLength:    uint8(iputil.GetPrefixLengthAsInt(p)),
		SpecLabels:      prefix.Labels,
	}
}

func BuildAllocationFromIPAllocation(cr *ipamv1alpha1.IPAllocation) *Allocation {
	if cr.Spec.Selector == nil {
		cr.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{},
		}
	}
	return &Allocation{
		NamespacedName: types.NamespacedName{
			Name:      cr.GetName(),
			Namespace: cr.GetNamespace(),
		},
		Origin:          ipamv1alpha1.OriginIPAllocation,
		NetworkInstance: cr.Spec.NetworkInstance,
		PrefixKind:      cr.Spec.PrefixKind,
		AddresFamily:    ipamv1alpha1.AddressFamily(cr.Spec.AddressFamily),
		Prefix:          cr.Spec.Prefix,
		PrefixLength:    cr.Spec.PrefixLength,
		//SubnetName:      cr.Spec.Selector.MatchLabels[ipamv1alpha1.NephioSubnetNameKey],
		SpecLabels:     cr.Spec.Labels,
		SelectorLabels: cr.Spec.Selector.MatchLabels,
	}
}
*/

/*
func (in *Allocation) DeepCopy() (*Allocation, error) {
	if in == nil {
		return nil, errors.New("in cannot be nil")
	}
	//fmt.Printf("json copy input %v\n", in)
	bytes, err := json.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal input data")
	}
	out := &Allocation{}
	err = json.Unmarshal(bytes, &out)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal to output data")
	}
	return out, nil
}
*/

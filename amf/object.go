package amf

type ObjectValue struct {
	value any
}

func (ov *ObjectValue) Get() any {
	return ov.value
}

func (ov *ObjectValue) Float64() float64 {
	if v, ok := ov.value.(float64); ok {
		return v
	}
	return 0
}

func (ov *ObjectValue) Uint32() uint32 {
	return uint32(ov.Float64())
}

func (ov *ObjectValue) Uint16() uint16 {
	return uint16(ov.Float64())
}

func (ov *ObjectValue) Uint8() uint8 {
	return uint8(ov.Float64())
}

func (ov *ObjectValue) String() string {
	if v, ok := ov.value.(string); ok {
		return v
	}
	return ""
}

func (ov *ObjectValue) Bool() bool {
	if v, ok := ov.value.(bool); ok {
		return v
	}
	return false
}

type ObjectProp struct {
	key   string
	value *ObjectValue
}

func (p *ObjectProp) Value() *ObjectValue {
	return p.value
}

func (p *ObjectProp) Key() string {
	return p.key
}

const blankPropKey = "__blank__"

type Object struct {
	data map[string]*ObjectProp
}

func NewObject() Object {
	return Object{
		data: map[string]*ObjectProp{
			blankPropKey: nil,
		},
	}
}

func (o *Object) InitIterator(itr *ObjectIterator) {
	keys := make([]string, 0, len(o.data))
	for key := range o.data {
		keys = append(keys, key)
	}

	itr.obj = o
	itr.keys = keys
	itr.index = 0
}

func (o *Object) HasProp(key string) bool {
	_, ok := o.data[key]
	return ok
}

func (o *Object) AddProp(key string, value any) {
	o.data[key] = &ObjectProp{key, &ObjectValue{value}}
}

func (o *Object) Prop(key string) *ObjectProp {
	if v, ok := o.data[key]; ok {
		return v
	}
	return o.data[blankPropKey]
}

type ObjectIterator struct {
	obj   *Object
	keys  []string
	index int
}

func (itr *ObjectIterator) Next() (string, *ObjectProp, bool) {
	if itr.index >= len(itr.obj.data) {
		return "", nil, false
	}

	key := itr.keys[itr.index]
	itr.index += 1

	if key == blankPropKey {
		return itr.Next()
	}
	return key, itr.obj.Prop(key), true
}

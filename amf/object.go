package amf

type ObjectValue struct {
	value any
}

func (ov ObjectValue) Get() any {
	return ov.value
}

func (ov ObjectValue) GetFloat64() float64 {
	if v, ok := ov.value.(float64); ok {
		return v
	}
	return 0
}

func (ov ObjectValue) GetString() string {
	if v, ok := ov.value.(string); ok {
		return v
	}
	return ""
}

func (ov ObjectValue) GetBool() bool {
	if v, ok := ov.value.(bool); ok {
		return v
	}
	return false
}

type ObjectProp struct {
	key   string
	value ObjectValue
}

func (p *ObjectProp) GetValue() ObjectValue {
	return p.value
}

func (p *ObjectProp) GetKey() string {
	return p.key
}

type Object map[string]ObjectProp

func NewObject() Object {
	return make(map[string]ObjectProp)
}

func (o Object) AddProp(key string, value any) {
	o[key] = ObjectProp{key, ObjectValue{value}}
}

func (o Object) GetProp(key string) (*ObjectProp, error) {
	if v, ok := o[key]; ok {
		return &v, nil
	}
	return nil, ErrNoObjProp
}

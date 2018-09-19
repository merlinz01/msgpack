package decoding

import (
	"encoding/binary"
	"reflect"

	"github.com/shamaton/msgpack/def"
)

var (
	typeMapStringInt = reflect.TypeOf(map[string]int{})
)

func (d *decoder) isFixMap(v byte) bool {
	return def.FixMap <= v && v <= def.FixMap+0x0f
}

func (d *decoder) mapLength(offset int, k reflect.Kind) (int, int, error) {
	code, offset := d.readSize1(offset)

	switch {
	case d.isFixMap(code):
		return int(code - def.FixMap), offset, nil
	case code == def.Map16:
		bs, offset := d.readSize2(offset)
		return int(binary.BigEndian.Uint16(bs)), offset, nil
	case code == def.Map32:
		bs, offset := d.readSize4(offset)
		return int(binary.BigEndian.Uint32(bs)), offset, nil
	}
	return 0, 0, d.errorTemplate(code, k)
}

func (d *decoder) asFixedMap(rv reflect.Value, offset int, l int) (int, bool, error) {
	t := rv.Type()

	keyKind := t.Key().Kind()
	valueKind := t.Elem().Kind()

	// todo : add types
	switch t {
	case typeMapStringInt:
		m := make(map[string]int, l)
		for i := 0; i < l; i++ {
			k, o, err := d.asString(offset, keyKind)
			if err != nil {
				return 0, false, err
			}
			v, o, err := d.asInt(o, valueKind)
			if err != nil {
				return 0, false, err
			}
			m[k] = int(v)
			offset = o
		}
		rv.Set(reflect.ValueOf(m))
		return offset, true, nil
	}

	return offset, false, nil
}

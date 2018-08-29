package decoding

import (
	"fmt"
	"reflect"

	"github.com/shamaton/msgpack/internal/common"
)

type decoder struct {
	data    []byte
	asArray bool
	common.Common
}

func Decode(data []byte, holder interface{}, asArray bool) error {
	d := decoder{data: data, asArray: asArray}

	rv := reflect.ValueOf(holder)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("holder must set pointer value. but got: %t", holder)
	}

	rv = rv.Elem()

	last, err := d.deserialize(rv, 0)
	if err != nil {
		return err
	}
	if len(data) != last {
		return fmt.Errorf("failed deserialization size=%d, last=%d", len(data), last)
	}
	return err
}

func (d *decoder) deserialize(rv reflect.Value, offset int) (int, error) {
	k := rv.Kind()
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, o, err := d.asInt(offset, k)
		if err != nil {
			return 0, err
		}
		rv.SetInt(v)
		offset = o

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, o, err := d.asUint(offset, k)
		if err != nil {
			return 0, err
		}
		rv.SetUint(v)
		offset = o

	case reflect.Float32:
		v, o, err := d.asFloat32(offset, k)
		if err != nil {
			return 0, err
		}
		rv.SetFloat(float64(v))
		offset = o

	case reflect.Float64:
		v, o, err := d.asFloat64(offset, k)
		if err != nil {
			return 0, err
		}
		rv.SetFloat(v)
		offset = o

	case reflect.String:
		v, o, err := d.asString(offset, k)
		if err != nil {
			return 0, err
		}
		rv.SetString(v)
		offset = o

	case reflect.Bool:
		v, o, err := d.asBool(offset, k)
		if err != nil {
			return 0, err
		}
		rv.SetBool(v)
		offset = o

	case reflect.Slice:
		// nil
		if d.isCodeNil(d.data[offset]) {
			offset++
			return offset, nil
		}
		// byte slice
		if d.isCodeBin(d.data[offset]) {
			bs, offset, err := d.asBin(offset, k)
			if err != nil {
				return 0, err
			}
			rv.SetBytes(bs)
			return offset, nil
		}
		// string to bytes
		if d.isCodeString(d.data[offset]) {
			l, offset, err := d.stringByteLength(offset, k)
			if err != nil {
				return 0, err
			}
			bs, offset := d.asStringByte(offset, l, k)
			rv.SetBytes(bs)
			return offset, nil
		}

		// get slice length
		l, o, err := d.sliceLength(offset, k)
		if err != nil {
			return 0, err
		}

		// check fixed type
		fixedOffset, found, err := d.asFixedSlice(rv, o, l)
		if err != nil {
			return 0, err
		}
		if found {
			return fixedOffset, nil
		}

		// create slice dynamically
		e := rv.Type().Elem()
		tmpSlice := reflect.MakeSlice(rv.Type(), l, l)
		for i := 0; i < l; i++ {
			v := reflect.New(e).Elem()
			o, err = d.deserialize(v, o)
			if err != nil {
				return 0, err
			}

			tmpSlice.Index(i).Set(v)
		}
		rv.Set(tmpSlice)
		offset = o

	case reflect.Array:
		// nil
		if d.isCodeNil(d.data[offset]) {
			offset++
			return offset, nil
		}
		// byte slice
		if d.isCodeBin(d.data[offset]) {
			// todo : length check
			bs, offset, err := d.asBin(offset, k)
			if err != nil {
				return 0, err
			}
			for i, b := range bs {
				rv.Index(i).SetUint(uint64(b))
			}
			return offset, nil
		}
		// string to bytes
		if d.isCodeString(d.data[offset]) {
			l, offset, err := d.stringByteLength(offset, k)
			if err != nil {
				return 0, err
			}
			if l > rv.Len() {
				return 0, fmt.Errorf("%v len is %d, but msgpack has %d elements", rv.Type(), rv.Len(), l)
			}
			bs, offset := d.asStringByte(offset, l, k)
			for i, b := range bs {
				rv.Index(i).SetUint(uint64(b))
			}
			return offset, nil
		}

		// get slice length
		l, o, err := d.sliceLength(offset, k)
		if err != nil {
			return 0, err
		}

		if l > rv.Len() {
			return 0, fmt.Errorf("%v len is %d, but msgpack has %d elements", rv.Type(), rv.Len(), l)
		}

		// create array dynamically
		for i := 0; i < l; i++ {
			o, err = d.deserialize(rv.Index(i), o)
			if err != nil {
				return 0, err
			}
		}
		offset = o

	case reflect.Map:
		// nil
		if d.isCodeNil(d.data[offset]) {
			offset++
			return offset, nil
		}

		// get map length
		l, o, err := d.mapLength(offset, k)
		if err != nil {
			return 0, err
		}

		// check fixed type
		fixedOffset, found, err := d.asFixedMap(rv, o, l)
		if err != nil {
			return 0, err
		}
		if found {
			return fixedOffset, nil
		}

		// create dynamically
		key := rv.Type().Key()
		value := rv.Type().Elem()
		if rv.IsNil() {
			rv.Set(reflect.MakeMap(rv.Type()))
		}
		for i := 0; i < l; i++ {
			k := reflect.New(key).Elem()
			v := reflect.New(value).Elem()
			o, err = d.deserialize(k, o)
			if err != nil {
				return 0, err
			}
			o, err = d.deserialize(v, o)
			if err != nil {
				return 0, err
			}

			rv.SetMapIndex(k, v)
		}
		offset = o

	case reflect.Struct:
		/*
			if d.isDateTime(offset) {
				dt, offset, err := d.asDateTime(offset, k)
				if err != nil {
					return 0, err
				}
				rv.Set(reflect.ValueOf(dt))
				return offset, nil
			}
		*/

		for i := range extCoders {
			if extCoders[i].IsType(offset, &d.data) {
				v, offset, err := extCoders[i].AsValue(offset, k, &d.data)
				if err != nil {
					return 0, err
				}
				rv.Set(reflect.ValueOf(v))
				return offset, nil
			}
		}

		if d.asArray {
			o, err := d.setStructFromArray(rv, offset, k)
			if err != nil {
				return 0, err
			}
			offset = o
		} else {
			o, err := d.setStructFromMap(rv, offset, k)
			if err != nil {
				return 0, err
			}
			offset = o
		}

	case reflect.Ptr:
		// nil
		if d.isCodeNil(d.data[offset]) {
			offset++
			return offset, nil
		}

		if rv.Elem().Kind() == reflect.Invalid {
			n := reflect.New(rv.Type().Elem())
			rv.Set(n)
		}

		o, err := d.deserialize(rv.Elem(), offset)
		if err != nil {
			return 0, err
		}
		offset = o

	case reflect.Interface:
		v, o, err := d.asInterface(offset, k)
		if err != nil {
			return 0, err
		}
		if v != nil {
			rv.Set(reflect.ValueOf(v))
		}
		offset = o

	default:
		return 0, d.errorTemplate(d.data[offset], k)
	}
	return offset, nil
}

func (d *decoder) errorTemplate(code byte, k reflect.Kind) error {
	return fmt.Errorf("msgpack : invalid code %x decoding %v", code, k)
}

package rtmp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// AMF0 data type markers
const (
	AMF0TypeNumber      byte = 0x00
	AMF0TypeBoolean     byte = 0x01
	AMF0TypeString      byte = 0x02
	AMF0TypeObject      byte = 0x03
	AMF0TypeNull        byte = 0x05
	AMF0TypeUndefined   byte = 0x06
	AMF0TypeECMAArray   byte = 0x08
	AMF0TypeObjectEnd   byte = 0x09
	AMF0TypeStrictArray byte = 0x0A
	AMF0TypeDate        byte = 0x0B
	AMF0TypeLongString  byte = 0x0C
)

// AMF0Encoder encodes data in AMF0 format
type AMF0Encoder struct {
	w io.Writer
}

// NewAMF0Encoder creates a new AMF0 encoder
func NewAMF0Encoder(w io.Writer) *AMF0Encoder {
	return &AMF0Encoder{w: w}
}

// EncodeNumber encodes a number (float64)
func (e *AMF0Encoder) EncodeNumber(n float64) error {
	if err := e.writeByte(AMF0TypeNumber); err != nil {
		return err
	}
	return binary.Write(e.w, binary.BigEndian, math.Float64bits(n))
}

// EncodeBoolean encodes a boolean
func (e *AMF0Encoder) EncodeBoolean(b bool) error {
	if err := e.writeByte(AMF0TypeBoolean); err != nil {
		return err
	}
	if b {
		return e.writeByte(0x01)
	}
	return e.writeByte(0x00)
}

// EncodeString encodes a string
func (e *AMF0Encoder) EncodeString(s string) error {
	if len(s) > 65535 {
		return e.EncodeLongString(s)
	}

	if err := e.writeByte(AMF0TypeString); err != nil {
		return err
	}
	if err := binary.Write(e.w, binary.BigEndian, uint16(len(s))); err != nil {
		return err
	}
	_, err := e.w.Write([]byte(s))
	return err
}

// EncodeLongString encodes a long string (>65535 characters)
func (e *AMF0Encoder) EncodeLongString(s string) error {
	if err := e.writeByte(AMF0TypeLongString); err != nil {
		return err
	}
	if err := binary.Write(e.w, binary.BigEndian, uint32(len(s))); err != nil {
		return err
	}
	_, err := e.w.Write([]byte(s))
	return err
}

// EncodeNull encodes null
func (e *AMF0Encoder) EncodeNull() error {
	return e.writeByte(AMF0TypeNull)
}

// EncodeObject encodes an object (map)
func (e *AMF0Encoder) EncodeObject(obj map[string]interface{}) error {
	if err := e.writeByte(AMF0TypeObject); err != nil {
		return err
	}

	for key, value := range obj {
		// Write property name (no type marker for property names)
		if err := binary.Write(e.w, binary.BigEndian, uint16(len(key))); err != nil {
			return err
		}
		if _, err := e.w.Write([]byte(key)); err != nil {
			return err
		}

		// Write property value
		if err := e.Encode(value); err != nil {
			return err
		}
	}

	// Object end marker
	if err := binary.Write(e.w, binary.BigEndian, uint16(0)); err != nil {
		return err
	}
	return e.writeByte(AMF0TypeObjectEnd)
}

// EncodeECMAArray encodes an ECMA array
func (e *AMF0Encoder) EncodeECMAArray(arr map[string]interface{}) error {
	if err := e.writeByte(AMF0TypeECMAArray); err != nil {
		return err
	}

	// Write array length
	if err := binary.Write(e.w, binary.BigEndian, uint32(len(arr))); err != nil {
		return err
	}

	for key, value := range arr {
		// Write property name
		if err := binary.Write(e.w, binary.BigEndian, uint16(len(key))); err != nil {
			return err
		}
		if _, err := e.w.Write([]byte(key)); err != nil {
			return err
		}

		// Write property value
		if err := e.Encode(value); err != nil {
			return err
		}
	}

	// Array end marker
	if err := binary.Write(e.w, binary.BigEndian, uint16(0)); err != nil {
		return err
	}
	return e.writeByte(AMF0TypeObjectEnd)
}

// Encode encodes any value
func (e *AMF0Encoder) Encode(v interface{}) error {
	if v == nil {
		return e.EncodeNull()
	}

	switch val := v.(type) {
	case float64:
		return e.EncodeNumber(val)
	case int:
		return e.EncodeNumber(float64(val))
	case int32:
		return e.EncodeNumber(float64(val))
	case int64:
		return e.EncodeNumber(float64(val))
	case uint32:
		return e.EncodeNumber(float64(val))
	case bool:
		return e.EncodeBoolean(val)
	case string:
		return e.EncodeString(val)
	case map[string]interface{}:
		return e.EncodeObject(val)
	default:
		return fmt.Errorf("unsupported AMF0 type: %T", v)
	}
}

func (e *AMF0Encoder) writeByte(b byte) error {
	_, err := e.w.Write([]byte{b})
	return err
}

// AMF0Decoder decodes AMF0 data
type AMF0Decoder struct {
	r io.Reader
}

// NewAMF0Decoder creates a new AMF0 decoder
func NewAMF0Decoder(r io.Reader) *AMF0Decoder {
	return &AMF0Decoder{r: r}
}

// Decode decodes the next AMF0 value
func (d *AMF0Decoder) Decode() (interface{}, error) {
	typeMarker, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch typeMarker {
	case AMF0TypeNumber:
		return d.DecodeNumber()
	case AMF0TypeBoolean:
		return d.DecodeBoolean()
	case AMF0TypeString:
		return d.DecodeString()
	case AMF0TypeObject:
		return d.DecodeObject()
	case AMF0TypeNull, AMF0TypeUndefined:
		return nil, nil
	case AMF0TypeECMAArray:
		return d.DecodeECMAArray()
	case AMF0TypeLongString:
		return d.DecodeLongString()
	default:
		return nil, fmt.Errorf("unsupported AMF0 type marker: 0x%02x", typeMarker)
	}
}

// DecodeNumber decodes a number
func (d *AMF0Decoder) DecodeNumber() (float64, error) {
	var bits uint64
	if err := binary.Read(d.r, binary.BigEndian, &bits); err != nil {
		return 0, err
	}
	return math.Float64frombits(bits), nil
}

// DecodeBoolean decodes a boolean
func (d *AMF0Decoder) DecodeBoolean() (bool, error) {
	b, err := d.readByte()
	if err != nil {
		return false, err
	}
	return b != 0, nil
}

// DecodeString decodes a string
func (d *AMF0Decoder) DecodeString() (string, error) {
	var length uint16
	if err := binary.Read(d.r, binary.BigEndian, &length); err != nil {
		return "", err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

// DecodeLongString decodes a long string
func (d *AMF0Decoder) DecodeLongString() (string, error) {
	var length uint32
	if err := binary.Read(d.r, binary.BigEndian, &length); err != nil {
		return "", err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

// DecodeObject decodes an object
func (d *AMF0Decoder) DecodeObject() (map[string]interface{}, error) {
	obj := make(map[string]interface{})

	for {
		// Read property name length
		var nameLen uint16
		if err := binary.Read(d.r, binary.BigEndian, &nameLen); err != nil {
			return nil, err
		}

		// Check for object end marker
		if nameLen == 0 {
			marker, err := d.readByte()
			if err != nil {
				return nil, err
			}
			if marker == AMF0TypeObjectEnd {
				break
			}
			return nil, fmt.Errorf("expected object end marker, got 0x%02x", marker)
		}

		// Read property name
		nameBuf := make([]byte, nameLen)
		if _, err := io.ReadFull(d.r, nameBuf); err != nil {
			return nil, err
		}
		name := string(nameBuf)

		// Read property value
		value, err := d.Decode()
		if err != nil {
			return nil, err
		}

		obj[name] = value
	}

	return obj, nil
}

// DecodeECMAArray decodes an ECMA array
func (d *AMF0Decoder) DecodeECMAArray() (map[string]interface{}, error) {
	// Read array length (not used, just for compatibility)
	var length uint32
	if err := binary.Read(d.r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	// ECMA array is essentially an object
	return d.DecodeObject()
}

func (d *AMF0Decoder) readByte() (byte, error) {
	buf := make([]byte, 1)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return 0, err
	}
	return buf[0], nil
}

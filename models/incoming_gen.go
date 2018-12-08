package models

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *BaseInfos) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "MailFrom":
			z.MailFrom, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "MailFrom")
				return
			}
		case "RcptTo":
			var zb0002 uint32
			zb0002, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "RcptTo")
				return
			}
			if cap(z.RcptTo) >= int(zb0002) {
				z.RcptTo = (z.RcptTo)[:zb0002]
			} else {
				z.RcptTo = make([]string, zb0002)
			}
			for za0001 := range z.RcptTo {
				z.RcptTo[za0001], err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "RcptTo", za0001)
					return
				}
			}
		case "Host":
			z.Host, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Family":
			z.Family, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Family")
				return
			}
		case "Port":
			z.Port, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Port")
				return
			}
		case "Addr":
			z.Addr, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Addr")
				return
			}
		case "Helo":
			z.Helo, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Helo")
				return
			}
		case "TimeReported":
			z.TimeReported, err = dc.ReadTime()
			if err != nil {
				err = msgp.WrapError(err, "TimeReported")
				return
			}
		case "UID":
			err = dc.ReadExactBytes((z.UID)[:])
			if err != nil {
				err = msgp.WrapError(err, "UID")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *BaseInfos) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 9
	// write "MailFrom"
	err = en.Append(0x89, 0xa8, 0x4d, 0x61, 0x69, 0x6c, 0x46, 0x72, 0x6f, 0x6d)
	if err != nil {
		return
	}
	err = en.WriteString(z.MailFrom)
	if err != nil {
		err = msgp.WrapError(err, "MailFrom")
		return
	}
	// write "RcptTo"
	err = en.Append(0xa6, 0x52, 0x63, 0x70, 0x74, 0x54, 0x6f)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.RcptTo)))
	if err != nil {
		err = msgp.WrapError(err, "RcptTo")
		return
	}
	for za0001 := range z.RcptTo {
		err = en.WriteString(z.RcptTo[za0001])
		if err != nil {
			err = msgp.WrapError(err, "RcptTo", za0001)
			return
		}
	}
	// write "Host"
	err = en.Append(0xa4, 0x48, 0x6f, 0x73, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.Host)
	if err != nil {
		err = msgp.WrapError(err, "Host")
		return
	}
	// write "Family"
	err = en.Append(0xa6, 0x46, 0x61, 0x6d, 0x69, 0x6c, 0x79)
	if err != nil {
		return
	}
	err = en.WriteString(z.Family)
	if err != nil {
		err = msgp.WrapError(err, "Family")
		return
	}
	// write "Port"
	err = en.Append(0xa4, 0x50, 0x6f, 0x72, 0x74)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Port)
	if err != nil {
		err = msgp.WrapError(err, "Port")
		return
	}
	// write "Addr"
	err = en.Append(0xa4, 0x41, 0x64, 0x64, 0x72)
	if err != nil {
		return
	}
	err = en.WriteString(z.Addr)
	if err != nil {
		err = msgp.WrapError(err, "Addr")
		return
	}
	// write "Helo"
	err = en.Append(0xa4, 0x48, 0x65, 0x6c, 0x6f)
	if err != nil {
		return
	}
	err = en.WriteString(z.Helo)
	if err != nil {
		err = msgp.WrapError(err, "Helo")
		return
	}
	// write "TimeReported"
	err = en.Append(0xac, 0x54, 0x69, 0x6d, 0x65, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	if err != nil {
		return
	}
	err = en.WriteTime(z.TimeReported)
	if err != nil {
		err = msgp.WrapError(err, "TimeReported")
		return
	}
	// write "UID"
	err = en.Append(0xa3, 0x55, 0x49, 0x44)
	if err != nil {
		return
	}
	err = en.WriteBytes((z.UID)[:])
	if err != nil {
		err = msgp.WrapError(err, "UID")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *BaseInfos) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 9
	// string "MailFrom"
	o = append(o, 0x89, 0xa8, 0x4d, 0x61, 0x69, 0x6c, 0x46, 0x72, 0x6f, 0x6d)
	o = msgp.AppendString(o, z.MailFrom)
	// string "RcptTo"
	o = append(o, 0xa6, 0x52, 0x63, 0x70, 0x74, 0x54, 0x6f)
	o = msgp.AppendArrayHeader(o, uint32(len(z.RcptTo)))
	for za0001 := range z.RcptTo {
		o = msgp.AppendString(o, z.RcptTo[za0001])
	}
	// string "Host"
	o = append(o, 0xa4, 0x48, 0x6f, 0x73, 0x74)
	o = msgp.AppendString(o, z.Host)
	// string "Family"
	o = append(o, 0xa6, 0x46, 0x61, 0x6d, 0x69, 0x6c, 0x79)
	o = msgp.AppendString(o, z.Family)
	// string "Port"
	o = append(o, 0xa4, 0x50, 0x6f, 0x72, 0x74)
	o = msgp.AppendInt(o, z.Port)
	// string "Addr"
	o = append(o, 0xa4, 0x41, 0x64, 0x64, 0x72)
	o = msgp.AppendString(o, z.Addr)
	// string "Helo"
	o = append(o, 0xa4, 0x48, 0x65, 0x6c, 0x6f)
	o = msgp.AppendString(o, z.Helo)
	// string "TimeReported"
	o = append(o, 0xac, 0x54, 0x69, 0x6d, 0x65, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	o = msgp.AppendTime(o, z.TimeReported)
	// string "UID"
	o = append(o, 0xa3, 0x55, 0x49, 0x44)
	o = msgp.AppendBytes(o, (z.UID)[:])
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *BaseInfos) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "MailFrom":
			z.MailFrom, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "MailFrom")
				return
			}
		case "RcptTo":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "RcptTo")
				return
			}
			if cap(z.RcptTo) >= int(zb0002) {
				z.RcptTo = (z.RcptTo)[:zb0002]
			} else {
				z.RcptTo = make([]string, zb0002)
			}
			for za0001 := range z.RcptTo {
				z.RcptTo[za0001], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "RcptTo", za0001)
					return
				}
			}
		case "Host":
			z.Host, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Host")
				return
			}
		case "Family":
			z.Family, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Family")
				return
			}
		case "Port":
			z.Port, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Port")
				return
			}
		case "Addr":
			z.Addr, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Addr")
				return
			}
		case "Helo":
			z.Helo, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Helo")
				return
			}
		case "TimeReported":
			z.TimeReported, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "TimeReported")
				return
			}
		case "UID":
			bts, err = msgp.ReadExactBytes(bts, (z.UID)[:])
			if err != nil {
				err = msgp.WrapError(err, "UID")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *BaseInfos) Msgsize() (s int) {
	s = 1 + 9 + msgp.StringPrefixSize + len(z.MailFrom) + 7 + msgp.ArrayHeaderSize
	for za0001 := range z.RcptTo {
		s += msgp.StringPrefixSize + len(z.RcptTo[za0001])
	}
	s += 5 + msgp.StringPrefixSize + len(z.Host) + 7 + msgp.StringPrefixSize + len(z.Family) + 5 + msgp.IntSize + 5 + msgp.StringPrefixSize + len(z.Addr) + 5 + msgp.StringPrefixSize + len(z.Helo) + 13 + msgp.TimeSize + 4 + msgp.ArrayHeaderSize + (16 * (msgp.ByteSize))
	return
}

// DecodeMsg implements msgp.Decodable
func (z *IncomingMail) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "BaseInfos":
			err = z.BaseInfos.DecodeMsg(dc)
			if err != nil {
				err = msgp.WrapError(err, "BaseInfos")
				return
			}
		case "Data":
			z.Data, err = dc.ReadBytes(z.Data)
			if err != nil {
				err = msgp.WrapError(err, "Data")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *IncomingMail) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "BaseInfos"
	err = en.Append(0x82, 0xa9, 0x42, 0x61, 0x73, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x73)
	if err != nil {
		return
	}
	err = z.BaseInfos.EncodeMsg(en)
	if err != nil {
		err = msgp.WrapError(err, "BaseInfos")
		return
	}
	// write "Data"
	err = en.Append(0xa4, 0x44, 0x61, 0x74, 0x61)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.Data)
	if err != nil {
		err = msgp.WrapError(err, "Data")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *IncomingMail) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "BaseInfos"
	o = append(o, 0x82, 0xa9, 0x42, 0x61, 0x73, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x73)
	o, err = z.BaseInfos.MarshalMsg(o)
	if err != nil {
		err = msgp.WrapError(err, "BaseInfos")
		return
	}
	// string "Data"
	o = append(o, 0xa4, 0x44, 0x61, 0x74, 0x61)
	o = msgp.AppendBytes(o, z.Data)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *IncomingMail) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "BaseInfos":
			bts, err = z.BaseInfos.UnmarshalMsg(bts)
			if err != nil {
				err = msgp.WrapError(err, "BaseInfos")
				return
			}
		case "Data":
			z.Data, bts, err = msgp.ReadBytesBytes(bts, z.Data)
			if err != nil {
				err = msgp.WrapError(err, "Data")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *IncomingMail) Msgsize() (s int) {
	s = 1 + 10 + z.BaseInfos.Msgsize() + 5 + msgp.BytesPrefixSize + len(z.Data)
	return
}
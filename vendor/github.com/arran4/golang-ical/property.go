package ics

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
)

type BaseProperty struct {
	IANAToken      string
	ICalParameters map[string][]string
	Value          string
}

type PropertyParameter interface {
	KeyValue(s ...interface{}) (string, []string)
}

type KeyValues struct {
	Key   string
	Value []string
}

func (kv *KeyValues) KeyValue(s ...interface{}) (string, []string) {
	return kv.Key, kv.Value
}

func WithCN(cn string) PropertyParameter {
	return &KeyValues{
		Key:   string(ParameterCn),
		Value: []string{cn},
	}
}

func WithRSVP(b bool) PropertyParameter {
	return &KeyValues{
		Key:   string(ParameterRsvp),
		Value: []string{strconv.FormatBool(b)},
	}
}

func (property *BaseProperty) serialize(w io.Writer) {
	b := bytes.NewBufferString("")
	fmt.Fprint(b, property.IANAToken)
	for k, vs := range property.ICalParameters {
		fmt.Fprint(b, ";")
		fmt.Fprint(b, k)
		fmt.Fprint(b, "=")
		for vi, v := range vs {
			if vi > 0 {
				fmt.Fprint(b, ",")
			}
			if strings.ContainsAny(v, ";:\\\",") {
				v = strings.Replace(v, "\"", "\\\"", -1)
				v = strings.Replace(v, "\\", "\\\\", -1)
			}
			fmt.Fprint(b, v)
		}
	}
	fmt.Fprint(b, ":")
	fmt.Fprint(b, property.Value)
	foldLine(b.String(), w)
}

// foldLine converts a line into multiple lines, where no line is longer than 75 octets (bytes)
// as described in https://tools.ietf.org/html/rfc5545#section-3.1
// Many runes are encoded with multiple bytes. foldLine ensures lines are not split mid-rune.
func foldLine(longLine string, w io.Writer) {
	octetsThisLine := 0

	for _, thisRune := range []rune(longLine) {
		octetsThisRune := len(string(thisRune))

		if octetsThisLine+octetsThisRune > 75 {
			_, err := fmt.Fprintf(w, "\r\n ")
			if err != nil {
				panic(err)
			}

			octetsThisLine = 1 // initial whitespace character after \r\n
		}

		octetsThisLine += octetsThisRune

		_, err := fmt.Fprintf(w, string(thisRune))
		if err != nil {
			panic(err)
		}
	}
	fmt.Fprint(w, "\r\n")
}

type IANAProperty struct {
	BaseProperty
}

var (
	propertyIanaTokenReg  *regexp.Regexp
	propertyParamNameReg  *regexp.Regexp
	propertyParamValueReg *regexp.Regexp
	propertyValueTextReg  *regexp.Regexp
)

func init() {
	var err error
	propertyIanaTokenReg, err = regexp.Compile("[A-Za-z0-9-]{1,}")
	if err != nil {
		log.Panicf("Failed to build regex: %v", err)
	}
	propertyParamNameReg = propertyIanaTokenReg
	propertyParamValueReg, err = regexp.Compile("^(?:\"(?:[^\"\\\\]|\\[\"nrt])*\"|[^,;\\\\:\"]*)")
	if err != nil {
		log.Panicf("Failed to build regex: %v", err)
	}
	propertyValueTextReg, err = regexp.Compile("^.*")
	if err != nil {
		log.Panicf("Failed to build regex: %v", err)
	}
}

type ContentLine string

func ParseProperty(contentLine ContentLine) *BaseProperty {
	r := &BaseProperty{
		ICalParameters: map[string][]string{},
	}
	tokenPos := propertyIanaTokenReg.FindIndex([]byte(contentLine))
	if tokenPos == nil {
		return nil
	}
	p := 0
	r.IANAToken = string(contentLine[p+tokenPos[0] : p+tokenPos[1]])
	p += tokenPos[1]
	for {
		if p >= len(contentLine) {
			return nil
		}
		switch rune(contentLine[p]) {
		case ':':
			return parsePropertyValue(r, string(contentLine), p+1)
		case ';':
			var np int
			r, np = parsePropertyParam(r, string(contentLine), p+1)
			if r == nil {
				return nil
			}
			p = np
		default:
			return nil
		}
	}
	return nil
}

func parsePropertyParam(r *BaseProperty, contentLine string, p int) (*BaseProperty, int) {
	tokenPos := propertyParamNameReg.FindIndex([]byte(contentLine[p:]))
	if tokenPos == nil {
		return nil, p
	}
	k, v := "", ""
	k = string(contentLine[p : p+tokenPos[1]])
	p += tokenPos[1]
	switch rune(contentLine[p]) {
	case '=':
		p += 1
	default:
		return nil, p
	}
	for {
		if p >= len(contentLine) {
			return nil, p
		}
		tokenPos = propertyParamValueReg.FindIndex([]byte(contentLine[p:]))
		if tokenPos == nil {
			return nil, p
		}
		v = string(contentLine[p+tokenPos[0] : p+tokenPos[1]])
		p += tokenPos[1]
		r.ICalParameters[k] = append(r.ICalParameters[k], v)
		switch rune(contentLine[p]) {
		case ',':
			p += 1
		default:
			return r, p
		}
	}
	return nil, p
}

func parsePropertyValue(r *BaseProperty, contentLine string, p int) *BaseProperty {
	tokenPos := propertyValueTextReg.FindIndex([]byte(contentLine[p:]))
	if tokenPos == nil {
		return nil
	}
	r.Value = string(contentLine[p : p+tokenPos[1]])
	return r
}

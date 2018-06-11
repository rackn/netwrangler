package util

import (
	"encoding/json"
	"strconv"
)

// Remarshal marshals src into a buf as JSON, then unmarshals that buf
// into dest.
func Remarshal(src, dest interface{}) error {
	buf, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, dest)
}

// Validator is the function signature that all checker functions must have.
type Validator func(e *Err, k string, v interface{}) (res interface{}, valid bool)

// ValidateUnsupp always fails due to an unsupported key
func ValidateUnsupp(e *Err, k string, v interface{}) (res interface{}, valid bool) {
	e.Errorf("Key %s is not supported", k)
	return v, false
}

// ValidateBool will attempt translate v into a boolean value.
func ValidateBool(e *Err, k string, v interface{}) (res, valid bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		switch val {
		case "0", "f", "false", "off":
			return false, true
		case "1", "t", "true", "on":
			return true, true
		}
	}
	e.Errorf("%s: Cannot cast %v to a boolean", k, v)
	return false, false
}

// ValidateInt will attempt to translate v into an int, and make sure
// it is between min and max.
func ValidateInt(e *Err, k string, v interface{}, min, max int) (res int, valid bool) {
	switch vv := v.(type) {
	case int:
		res = vv
	case uint:
		res = int(vv)
	case int64:
		res = int(vv)
	case uint64:
		res = int(vv)
	case float64:
		res = int(vv)
	case string:
		vvs, err := strconv.ParseInt(vv, 0, 64)
		if err != nil {
			e.Errorf("%s: Cannot cast %v to an int: %v", k, v, err)
			return
		}
		res = int(vvs)
	default:
		e.Errorf("%s: Cannot cast %T(%v) to an %T(%v)", k, v, v, vv, vv)
		return
	}
	if valid = (min <= res && res <= max); !valid {
		e.Errorf("%s: %d out of range %d:%d", k, res, min, max)
	}
	return
}

// ValidateStrIn validates that v is a string, and (if any are passed)
// that is is one of a known set of values.
func ValidateStrIn(e *Err, k string, v interface{}, vals ...string) (res string, valid bool) {
	res, valid = v.(string)
	if !valid {
		e.Errorf("%s: %v is not a string", k, v)
		return
	}
	if valid = len(vals) == 0; !valid {
		for _, s := range vals {
			if valid = res == s; valid {
				return
			}
		}
		e.Errorf("%s: %s: Not in valid set: %v", k, res, valid)
	}
	return
}

// ValidateMac validates that v is a hardware address
func ValidateMac(e *Err, k string, v interface{}) (res HardwareAddr, valid bool) {
	res, valid = v.(HardwareAddr)
	if !valid {
		if err := Remarshal(v, &res); err != nil {
			e.Errorf("%s: Cannot cast %v to a HardwareAddr:%v", k, v, err)
			return
		}
		valid = true
	}
	return
}

// ValidateIP validates that v is an IP address or address range in CIDR format.
func ValidateIP(e *Err, k string, v interface{}) (res *IP, valid bool) {
	res, valid = v.(*IP)
	if !valid {
		if err := Remarshal(v, &res); err != nil {
			e.Errorf("%s: Cannot cast %v to an IP: %v", k, v, err)
			return
		}
		valid = true
	}
	return
}

// ValidateIPList validates that v can be represented as a list of *IP
// objects, and that they are all either CIDR addresses or not.
func ValidateIPList(e *Err, k string, v interface{}, cidr bool) (res []*IP, valid bool) {
	res, valid = v.([]*IP)
	if !valid {
		if err := Remarshal(v, &res); err != nil {
			valid = false
			e.Errorf("%s: Cannot cast %v to a list of IPs: %v", k, v, err)
			return
		}
		valid = true
	}
	for _, addr := range res {
		if addr.IsCIDR() == cidr {
			continue
		}
		valid = false
		e.Errorf("%s: %v is not in the expected format", k, addr)
	}
	return
}

// Check carries around a validator and a default value to be used when checking things.
type Check struct {
	d interface{}
	c Validator
	k string
	v func(interface{}) interface{}
}

// Validate validates that the passed-in v is valid.  It returns a new
// value, and whether or not the value should be used.  If v is not valid, an informative error
// will be added to e
func (c *Check) Validate(e *Err, k string, v interface{}) (interface{}, bool) {
	return c.c(e, k, v)
}

func (c *Check) keyName(n string) string {
	if c.k == "" {
		return n
	}
	return c.k
}

// D updates the default value for a Check to dfl.
func (c *Check) D(dfl interface{}) *Check {
	c.d = dfl
	return c
}

// C updates the validation function for a Check to checker.
func (c *Check) C(checker Validator) *Check {
	c.c = checker
	return c
}

// K updates the new field name for a checker to name.
func (c *Check) K(name string) *Check {
	c.k = name
	return c
}

// V updates the value translator with a new value translation function.
func (c *Check) V(f func(interface{}) interface{}) *Check {
	c.v = f
	return c
}

// D creates a new Check with a default value and a validator.
func D(defl interface{}, checker Validator) *Check {
	return &Check{d: defl, c: checker}
}

// C creates a new Check with no default value and a validator
func C(checker Validator) *Check {
	return &Check{c: checker}
}

// ValidateAndMarshal checks that m is valid according to checks
// (filling in any default values along the way), and if it is
// marshals the checked values in to val.
func ValidateAndMarshal(e *Err, vals interface{}, checks map[string]*Check, val interface{}) bool {
	m, ok := vals.(map[string]interface{})
	if !ok {
		e.Errorf("cannot validate format %T", vals)
		return false
	}
	res := map[string]interface{}{}
	resOK := true
	for key, check := range checks {
		v, found := m[key]
		if !found {
			if check.d != nil {
				res[check.keyName(key)] = check.d
			}
			continue
		}
		nv, valid := check.Validate(e, key, v)
		if !valid {
			resOK = false
			continue
		}
		if check.v != nil {
			nv = check.v(nv)
		}
		res[check.keyName(key)] = nv
	}
	if resOK {
		err := Remarshal(res, val)
		if err != nil {
			e.Errorf("Error converting to %T: %v", val, err)
			resOK = false
		}
	}
	return resOK
}

// VB returns a Validator that will validate boolean-ish values.
func VB() Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		return ValidateBool(e, k, v)
	}
}

// VI returns a Validator that will validate int-ish values that must
// be in a certian range.
func VI(min, max int) Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		return ValidateInt(e, k, v, min, max)
	}
}

// VS returns a Validator that will validate string values that must
// be one of a few set values.
func VS(r ...string) Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		return ValidateStrIn(e, k, v, r...)
	}
}

// VSS returns a Validator that will validate that all the strings
// in a slice of strings are in a set of specified values
func VSS(rs ...string) Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		res := []string{}
		resOK := true
		if err := Remarshal(v, &res); err != nil {
			e.Errorf("%s: Failed to translate %v into a string slice: %v", k, v, err)
			return nil, false
		}
		for _, sv := range res {
			if _, ok := ValidateStrIn(e, k, sv, rs...); !ok {
				resOK = false
			}
		}
		return res, resOK
	}
}

// VIP validates that v is an IP.
func VIP() Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		return ValidateIP(e, k, v)
	}
}

// VIP4 returns a Validator that will validate that the passed object
// represents an IP with an IPv4 address.
func VIP4() Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		res, valid := ValidateIP(e, k, v)
		valid = valid && res.IP.To4() != nil
		return res, valid
	}
}

// VIP6 returns a Validator that will validate that the passed object
// represents an IP with an IPv6 address.
func VIP6() Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		res, valid := ValidateIP(e, k, v)
		valid = valid && res.IP.To4() == nil
		return res, valid
	}
}

// VIPS returns a Validator that will validate a list of IP addresses,
// that must either all be CIDR formatted or bare addresses.
func VIPS(cidr bool) Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		return ValidateIPList(e, k, v, cidr)
	}
}

// VMAC returns a Validator that will validate that the passed object
// represents a HardwareAddr
func VMAC() Validator {
	return func(e *Err, k string, v interface{}) (interface{}, bool) {
		return ValidateMac(e, k, v)
	}
}

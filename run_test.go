package netwrangler

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	yaml "github.com/ghodss/yaml"
	gnet "github.com/rackn/gohai/plugins/net"
	"github.com/rackn/netwrangler/util"
)

func m(s string) gnet.HardwareAddr {
	res := &gnet.HardwareAddr{}
	if err := res.UnmarshalText([]byte(s)); err != nil {
		log.Panicf("Bad Mac %s: %v", s, err)
	}
	return *res
}

var testPhys = []util.Phy{
	{Interface: gnet.Interface{Name: "enp9s5", Driver: "foobar2000", HardwareAddr: m("de:ad:be:ef:ca:fe")}},
	{Interface: gnet.Interface{Name: "enp0s25", Driver: "broadcom", HardwareAddr: m("52:54:01:23:00:00")}},
	{Interface: gnet.Interface{Name: "enp1s0", Driver: "e1000", HardwareAddr: m("52:54:01:23:00:01")}},
	{Interface: gnet.Interface{Name: "enp2s0", Driver: "e1000", HardwareAddr: m("52:54:01:23:00:02")}},
	{Interface: gnet.Interface{Name: "enp3s0", Driver: "e1000", HardwareAddr: m("52:54:01:23:00:03")}},
	{Interface: gnet.Interface{Name: "enp4s0", Driver: "e1000", HardwareAddr: m("52:54:01:23:00:04")}},
	{Interface: gnet.Interface{Name: "enp5s0", Driver: "e1000", HardwareAddr: m("52:54:01:23:00:05")}},
	{Interface: gnet.Interface{Name: "enp6s0", Driver: "e1000", HardwareAddr: m("52:54:01:23:00:06")}},
	{Interface: gnet.Interface{Name: "ens3", Driver: "realtek", HardwareAddr: m("52:54:01:23:00:07")}},
	{Interface: gnet.Interface{Name: "ens5", Driver: "realtek", HardwareAddr: m("52:54:01:23:00:08")}},
	{
		Interface: gnet.Interface{Name: "eno1", Driver: "realtek", HardwareAddr: m("52:54:01:23:00:09")},
		BootIf:    true,
	},
}

func diff(expect, actual string) (string, error) {
	cmd := exec.Command("diff", "-Ndur", expect, actual)
	res, err := cmd.CombinedOutput()
	return string(res), err
}

func cmpOut(t *testing.T, actual, expect string) {
	out, err := diff(expect, actual)
	if out != "" {
		t.Errorf("ERROR: %s: diff from expected %s:\n%s", actual, expect, out)
	} else if err != nil {
		t.Errorf("ERROR: running diff: %v", err)
	} else {
		t.Logf("%s: No diff from expected: %s", actual, expect)
	}
}

func testRun(t *testing.T, loc, in, out string, wantErr bool) {
	//t.Helper()
	var (
		phys []util.Phy
		err  error
	)
	pwd, _ := os.Getwd()
	if err = os.Chdir(loc); err != nil {
		t.Errorf("Failed to change dir to %s: %v", loc, err)
		return
	}
	defer os.Chdir(pwd)
	args := []string{"test"}
	if st, ok := os.Stat("phys.yaml"); ok == nil && st.Mode().IsRegular() {
		phys, err = GatherPhysFromFile("phys.yaml")
		if err != nil {
			t.Error(err)
			return
		}
	} else {
		phys = testPhys
	}
	if wantErr {
		if f, e := os.Create("wantErr"); e == nil {
			f.Close()
		}
	} else {
		os.RemoveAll("wantErr")
	}
	os.RemoveAll("untouched")
	actualOut := path.Join(out, "actual")
	expectOut := path.Join(out, "expect")
	actualErr := path.Join(out, "actualErr")
	expectErr := path.Join(out, "expectErr")
	os.RemoveAll(actualOut)
	os.RemoveAll(actualErr)
	os.MkdirAll(out, 0755)
	args = append(args,
		"-op", "compile",
		"-in", in,
		"-src", in+".yaml",
		"-out", out,
		"-dest", actualOut)
	if strings.HasSuffix(loc, "-bindMacs") {
		args = append(args, "-bindMacs")
	}
	t.Logf("Running with args %v", args)
	if err = Compile(phys, in, out, in+".yaml", actualOut, strings.HasSuffix(loc, "-bindMacs")); err != nil {
		if !wantErr {
			t.Errorf("ERROR: loc %s: Unexpected error!\n%v", loc, err)
		}
		ioutil.WriteFile(actualErr, []byte(err.Error()), 0644)
		cmpOut(t, actualErr, expectErr)
	} else if wantErr {
		t.Errorf("ERROR: loc %s: No error!", loc)
	} else {
		cmpOut(t, actualOut, expectOut)
	}
}

func rt(t *testing.T, loc string, wantErr bool) {
	t.Helper()
	for _, in := range []string{"netplan"} {
		for _, out := range []string{"internal", "systemd", "rhel"} {
			testRun(t, loc, in, out, wantErr)
		}
	}
}

func TestPhys(t *testing.T) {
	buf, err := yaml.Marshal(testPhys)
	if err != nil {
		t.Errorf("Error marshalling phys: %v", err)
		return
	} else {
		t.Logf("orig: %s", string(buf))
	}
	np := []util.Phy{}
	if err := yaml.Unmarshal(buf, &np); err != nil {
		t.Errorf("Error unmarshalling phys: %v", err)
		return
	} else if !reflect.DeepEqual(testPhys, np) {
		t.Errorf("Unmarshalled phys not equal to phys: %v", err)
		b2, _ := yaml.Marshal(np)

		t.Errorf("new: %s", string(b2))
	}
}

func TestNetMangler(t *testing.T) {
	tests, err := filepath.Glob(path.Join("test-data", "*"))
	if err != nil {
		t.Errorf("FATAL: Error getting tests: %v", err)
		return
	}
	sort.Strings(tests)
	fails := map[string]bool{
		"test-data/direct_connect_gateway": true,
		"test-data/loopback_interface":     true,
		"test-data/wireless":               true,
	}
	for _, testPath := range tests {
		if st, err := os.Stat(testPath); err == nil && !st.IsDir() {
			continue
		}
		fail := fails[testPath]
		t.Logf("Testing %s, expect failure: %v", testPath, fail)
		rt(t, testPath, fail)
	}
}

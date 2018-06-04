package netmangler

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"testing"
)

func m(s string) HardwareAddr {
	mac, err := net.ParseMAC(s)
	if err != nil {
		log.Panicf("Bad Mac %s: %v", s, err)
	}
	return HardwareAddr(mac)
}

var testPhys = []Phy{
	{Name: "enp0s25", Driver: "broadcom", HwAddr: m("52:54:01:23:00:00")},
	{Name: "enp1s0", Driver: "e1000", HwAddr: m("52:54:01:23:00:01")},
	{Name: "enp2s0", Driver: "e1000", HwAddr: m("52:54:01:23:00:02")},
	{Name: "enp3s0", Driver: "e1000", HwAddr: m("52:54:01:23:00:03")},
	{Name: "enp4s0", Driver: "e1000", HwAddr: m("52:54:01:23:00:04")},
	{Name: "enp5s0", Driver: "e1000", HwAddr: m("52:54:01:23:00:05")},
	{Name: "enp6s0", Driver: "e1000", HwAddr: m("52:54:01:23:00:06")},
	{Name: "ens3", Driver: "realtek", HwAddr: m("52:54:01:23:00:07")},
	{Name: "ens5", Driver: "realtek", HwAddr: m("52:54:01:23:00:08")},
	{Name: "eno1", Driver: "realtek", HwAddr: m("52:54:01:23:00:09")},
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
	pwd, _ := os.Getwd()
	if err := os.Chdir(loc); err != nil {
		t.Errorf("Failed to change dir to %s: %v", loc, err)
		return
	}
	defer os.Chdir(pwd)
	args := []string{"test"}
	if st, ok := os.Stat("phys.yaml"); ok == nil && st.Mode().IsRegular() {
		args = append(args, "-phys", path.Join(loc, "phys.yaml"))
	} else {
		phys = testPhys
	}
	claimedPhys = map[string]Interface{}
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
	t.Logf("Running with args %v", args)
	if err := Run(args...); err != nil {
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
		for _, out := range []string{"layout"} {
			testRun(t, loc, in, out, wantErr)
		}
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
		"test-data/wireless":               true,
	}
	for _, testPath := range tests {
		fail := fails[testPath]
		t.Logf("Testing %s, expect failure: %v", testPath, fail)
		rt(t, testPath, fail)
	}
}

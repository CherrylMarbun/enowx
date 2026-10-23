package display

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDisplayServesItsClientAndShowsWindows(t *testing.T) {
	if ok, why := Available(); !ok {
		t.Skip(why)
	}
	d, err := Start(2)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Stop()
	if d.Number < firstNumber || !strings.HasPrefix(d.Env()[0], "DISPLAY=:") || !slices.Contains(d.Env(), "GDK_SCALE=2") {
		t.Fatalf("display %+v", d)
	}
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(d.Port) + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(strings.ToLower(string(page)), "<html") {
		t.Fatalf("the client page: %d %q", resp.StatusCode, page[:min(len(page), 300)])
	}
	// A program given the display opens its windows there.
	if _, err := exec.LookPath("xdpyinfo"); err == nil {
		cmd := exec.Command("xdpyinfo")
		cmd.Env = append(os.Environ(), d.Env()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("xdpyinfo on %s: %v: %s", d.Env(), err, out)
		}
	}
	d.Stop()
	time.Sleep(time.Second)
	if _, err := http.Get("http://127.0.0.1:" + strconv.Itoa(d.Port) + "/"); err == nil {
		t.Fatal("the display outlived Stop")
	}
}

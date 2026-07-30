package integration

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/network/dimse"
	"github.com/cocosip/go-dicom/pkg/network/server"
	"github.com/cocosip/go-dicom/pkg/network/status"
)

func TestEchoAndSendCommandsUseLocalDIMSEPeer(t *testing.T) {
	port := reserveDIMSEPort(t)
	var echoes, stores atomic.Int32
	srv := server.New(server.WithPort(port))
	srv.SetCEchoHandler(func(_ context.Context, request *dimse.CEchoRequest) (*dimse.CEchoResponse, error) {
		echoes.Add(1)
		return dimse.NewCEchoResponseFromRequest(request, status.Success), nil
	})
	srv.SetCStoreHandler(func(_ context.Context, request *dimse.CStoreRequest) (*dimse.CStoreResponse, error) {
		stores.Add(1)
		return dimse.NewCStoreResponseFromRequest(request, status.Success), nil
	})
	stopServer := startDIMSEPeer(t, srv)
	defer stopServer()

	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	targetArgs := []string{"--host", "127.0.0.1", "--port", stringPort(port), "--calling-ae", "DICOMCLI", "--called-ae", "LOCAL_SCP"}
	if output, err := runCLI(root, append([]string{"echo", "--json"}, targetArgs...)...); err != nil || !strings.Contains(output, `"status":"success"`) {
		t.Fatalf("echo output=%q err=%v", output, err)
	}
	if got := echoes.Load(); got != 1 {
		t.Fatalf("C-ECHO calls = %d, want 1", got)
	}
	if output, err := runCLI(root, append([]string{"send", "--json"}, append(targetArgs, fixtures.SingleFrame)...)...); err != nil || !strings.Contains(output, `"processed":1`) {
		t.Fatalf("send output=%q err=%v", output, err)
	}
	if got := stores.Load(); got != 1 {
		t.Fatalf("C-STORE calls = %d, want 1", got)
	}
}

func TestSendReadsStdinAndRollsAssociationsAtConfiguredMaximum(t *testing.T) {
	port := reserveDIMSEPort(t)
	var stores, closed atomic.Int32
	srv := server.New(server.WithPort(port))
	srv.SetCStoreHandler(func(_ context.Context, request *dimse.CStoreRequest) (*dimse.CStoreResponse, error) {
		stores.Add(1)
		return dimse.NewCStoreResponseFromRequest(request, status.Success), nil
	})
	srv.SetConnectionLifecycleHandlerFuncs(nil, func(context.Context, error) { closed.Add(1) })
	stopServer := startDIMSEPeer(t, srv)
	defer stopServer()

	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"send", "-", "--host", "127.0.0.1", "--port", stringPort(port), "--calling-ae", "DICOMCLI", "--called-ae", "LOCAL_SCP"}
	stdin := fixtures.SingleFrame + "\n" + fixtures.MultiFrame + "\n"
	if output, err := runCLIWithInput(root, stdin, args...); err != nil || !strings.Contains(output, "processed=2") {
		t.Fatalf("send stdin output=%q err=%v", output, err)
	}
	waitForCount(t, &closed, 1)
	if got := stores.Load(); got != 2 {
		t.Fatalf("C-STORE calls = %d, want 2", got)
	}

	if output, err := runCLIWithInput(root, stdin, append(args, "--max-instances", "1")...); err != nil || !strings.Contains(output, "processed=2") {
		t.Fatalf("send max-instances output=%q err=%v", output, err)
	}
	waitForCount(t, &closed, 3)
	if got := stores.Load(); got != 4 {
		t.Fatalf("C-STORE calls = %d, want 4", got)
	}
}

func TestSendDoesNotRetryPACSStatusAndReusesFailedListAsStdin(t *testing.T) {
	port := reserveDIMSEPort(t)
	var reject atomic.Bool
	reject.Store(true)
	srv := server.New(server.WithPort(port))
	srv.SetCStoreHandler(func(_ context.Context, request *dimse.CStoreRequest) (*dimse.CStoreResponse, error) {
		if reject.Load() {
			return dimse.NewCStoreResponseFromRequest(request, status.CStoreErrorCannotUnderstand), nil
		}
		return dimse.NewCStoreResponseFromRequest(request, status.Success), nil
	})
	stopServer := startDIMSEPeer(t, srv)
	defer stopServer()

	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(t.TempDir(), "send-report.json")
	failedListPath := filepath.Join(t.TempDir(), "failed.txt")
	targetArgs := []string{"--host", "127.0.0.1", "--port", stringPort(port), "--calling-ae", "DICOMCLI", "--called-ae", "LOCAL_SCP"}
	args := append([]string{"send", fixtures.SingleFrame, "--retries", "3", "--report", reportPath, "--failed-list", failedListPath}, targetArgs...)
	if output, err := runCLI(root, args...); err == nil || !strings.Contains(output, "failed=1") {
		t.Fatalf("failed send output=%q err=%v", output, err)
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), `"attempts": 1`) {
		t.Fatalf("PACS status was retried: %s", report)
	}
	failedList, err := os.ReadFile(failedListPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(failedList)) != fixtures.SingleFrame {
		t.Fatalf("failed list = %q, want %q", failedList, fixtures.SingleFrame)
	}

	reject.Store(false)
	if output, err := runCLIWithInput(root, string(failedList), append([]string{"send", "-"}, targetArgs...)...); err != nil || !strings.Contains(output, "processed=1") {
		t.Fatalf("retry from stdin output=%q err=%v", output, err)
	}
}

func TestSendHonorsAssociationConcurrencyLimit(t *testing.T) {
	port := reserveDIMSEPort(t)
	var active, maximum atomic.Int32
	srv := server.New(server.WithPort(port))
	srv.SetCStoreHandler(func(_ context.Context, request *dimse.CStoreRequest) (*dimse.CStoreResponse, error) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if observed >= current || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		active.Add(-1)
		return dimse.NewCStoreResponseFromRequest(request, status.Success), nil
	})
	stopServer := startDIMSEPeer(t, srv)
	defer stopServer()

	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"send", "-", "--max-instances", "1", "--concurrency", "2", "--host", "127.0.0.1", "--port", stringPort(port), "--calling-ae", "DICOMCLI", "--called-ae", "LOCAL_SCP"}
	input := fixtures.SingleFrame + "\n" + fixtures.MultiFrame + "\n"
	if output, err := runCLIWithInput(root, input, args...); err != nil || !strings.Contains(output, "processed=2") {
		t.Fatalf("concurrent send output=%q err=%v", output, err)
	}
	if got := maximum.Load(); got != 2 {
		t.Fatalf("maximum parallel C-STORE operations = %d, want 2", got)
	}
}

func reserveDIMSEPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func startDIMSEPeer(t *testing.T, srv *server.Server) func() {
	t.Helper()
	serverContext, cancelServer := context.WithCancel(context.Background())
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- srv.ListenAndServe(serverContext) }()
	deadline := time.Now().Add(time.Second)
	for !srv.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !srv.IsRunning() {
		cancelServer()
		t.Fatal("local DIMSE peer did not start")
	}
	return func() {
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), time.Second)
		defer cancelShutdown()
		_ = srv.Shutdown(shutdownContext)
		cancelServer()
		<-serverErrors
	}
}

func stringPort(port int) string {
	return strconv.Itoa(port)
}

func runCLIWithInput(root, input string, args ...string) (string, error) {
	command := exec.Command("go", append([]string{"run", "./cmd/dicom-cli"}, args...)...)
	command.Dir = root
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
	return string(output), err
}

func waitForCount(t *testing.T, value *atomic.Int32, expected int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for value.Load() < expected && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := value.Load(); got != expected {
		t.Fatalf("connection closes = %d, want %d", got, expected)
	}
}

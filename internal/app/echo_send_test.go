package app

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cocosip/dicom-cli/internal/config"
	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	godimse "github.com/cocosip/go-dicom/pkg/network/dimse"
	"github.com/cocosip/go-dicom/pkg/network/server"
	"github.com/cocosip/go-dicom/pkg/network/status"
)

func TestEchoAndStoreUseGoDicomClientAgainstLocalPeer(t *testing.T) {
	port := reservePort(t)
	var echoes, stores atomic.Int32
	srv := server.New(server.WithPort(port))
	srv.SetCEchoHandler(func(_ context.Context, request *godimse.CEchoRequest) (*godimse.CEchoResponse, error) {
		echoes.Add(1)
		return godimse.NewCEchoResponseFromRequest(request, status.Success), nil
	})
	srv.SetCStoreHandler(func(_ context.Context, request *godimse.CStoreRequest) (*godimse.CStoreResponse, error) {
		stores.Add(1)
		return godimse.NewCStoreResponseFromRequest(request, status.Success), nil
	})
	serverContext, stopServer := startLocalServer(t, srv)
	defer stopServer()

	target := config.PACSTarget{
		Host:           "127.0.0.1",
		Port:           port,
		CallingAETitle: "DICOMCLI",
		CalledAETitle:  "LOCAL_SCP",
		Timeouts: config.Timeouts{
			Connect:   time.Second,
			Associate: time.Second,
			Idle:      time.Second,
		},
	}
	if err := Echo(serverContext, target); err != nil {
		t.Fatalf("Echo() error = %v", err)
	}
	if got := echoes.Load(); got != 1 {
		t.Fatalf("C-ECHO calls = %d, want 1", got)
	}

	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	instance, err := ReadInstance(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(serverContext, target, []Instance{instance})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = session.Close() }()
	association := session.client.GetAssociation()
	context := association.FindPresentationContextByAbstractSyntax(instance.SOPClass)
	if context == nil || context.AcceptedTransferSyntax == nil {
		t.Fatalf("storage presentation context = %#v", context)
	}
	if got := context.AcceptedTransferSyntax.UID().String(); got != instance.TransferSyntax {
		t.Fatalf("accepted transfer syntax = %q, want original %q", got, instance.TransferSyntax)
	}
	if err := session.Store(serverContext, instance); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if got := stores.Load(); got != 1 {
		t.Fatalf("C-STORE calls = %d, want 1", got)
	}
}

func TestReadInstancePreservesOriginalTransferSyntax(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	instance, err := ReadInstance(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if instance.TransferSyntax != parsed.TransferSyntax.UID().String() {
		t.Fatalf("TransferSyntax = %q, want %q", instance.TransferSyntax, parsed.TransferSyntax.UID().String())
	}
}

func TestClientUsesConfiguredDIMSETimeouts(t *testing.T) {
	target := config.PACSTarget{Timeouts: config.Timeouts{
		Connect:   2 * time.Second,
		Associate: 3 * time.Second,
		Idle:      4 * time.Second,
	}}
	client := newClient(target)
	settings := client.GetConfig()
	if settings.ConnectTimeout != target.Timeouts.Connect || settings.AssociationTimeout != target.Timeouts.Associate || settings.RequestTimeout != target.Timeouts.Idle {
		t.Fatalf("client timeouts = %#v", settings)
	}
}

func TestRetryableRejectsPACSStatusAndAcceptsNetworkTimeout(t *testing.T) {
	if Retryable(errors.New("C-STORE failed with status: 0xA700")) {
		t.Fatal("PACS C-STORE status failure is retryable")
	}
	if !Retryable(timeoutError{}) {
		t.Fatal("network timeout is not retryable")
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "network timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func reservePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func startLocalServer(t *testing.T, srv *server.Server) (context.Context, func()) {
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
	return serverContext, func() {
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), time.Second)
		defer cancelShutdown()
		_ = srv.Shutdown(shutdownContext)
		cancelServer()
		<-serverErrors
	}
}

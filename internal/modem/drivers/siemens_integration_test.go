//go:build integration

package drivers

import (
	"context"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
	"github.com/legoser/gsm2mqtt/internal/transport"
)

type loggingPort struct {
	transport.Port
	t *testing.T
}

func (lp *loggingPort) Read(p []byte) (int, error) {
	n, err := lp.Port.Read(p)
	if n > 0 {
		lp.t.Logf("RX (%d bytes): %q", n, string(p[:n]))
	}
	if err != nil {
		lp.t.Logf("Read returned error: %v (n=%d)", err, n)
	}
	return n, err
}

func (lp *loggingPort) Write(p []byte) (int, error) {
	lp.t.Logf("TX (%d bytes): %q", len(p), string(p))
	return lp.Port.Write(p)
}

func TestSiemensMC35i_Hardware(t *testing.T) {
	portPath := "/dev/ttyUSB0"
	opener := transport.NewSerialOpener()
	rawPort, err := opener.Open(transport.PortConfig{
		Device:   portPath,
		BaudRate: 9600,
		DataBits: 8,
		StopBits: 1,
		Parity:   "none",
	})
	if err != nil {
		t.Skipf("skipping hardware test, unable to open %s: %v", portPath, err)
	}
	defer rawPort.Close()

	port := &loggingPort{Port: rawPort, t: t}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	engineCtx, engineCancel := context.WithCancel(context.Background())
	defer engineCancel()

	engine := at.NewEngine(port)
	go func() {
		err := engine.Start(engineCtx)
		t.Logf("engine.Start exited with: %v", err)
	}()

	// 1. Detect
	mType, err := modem.Detect(ctx, engine)
	if err != nil {
		t.Fatalf("modem detection error: %v", err)
	}
	t.Logf("Detected modem type: %s", mType)
	if mType != modem.TypeSiemens {
		t.Errorf("expected TypeSiemens, got %s", mType)
	}

	// 2. Init Siemens driver
	driver := NewSiemensDriver(engine)
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("driver.Init failed: %v", err)
	}
	t.Log("driver.Init succeeded")

	// 3. Identify
	info, err := driver.Identify()
	if err != nil {
		t.Fatalf("driver.Identify failed: %v", err)
	}
	t.Logf("Hardware Info: Manufacturer=%s Model=%s Revision=%s IMEI=%s",
		info.Manufacturer, info.Model, info.Revision, info.IMEI)

	// 4. Signal quality
	csq, err := driver.SignalQuality()
	if err != nil {
		t.Fatalf("driver.SignalQuality failed: %v", err)
	}
	t.Logf("Signal Quality CSQ: %d (RSSI: %d dBm)", csq, -113+2*csq)

	// 5. Storage
	storage, err := driver.StorageCapacity()
	if err != nil {
		t.Fatalf("driver.StorageCapacity failed: %v", err)
	}
	t.Logf("SMS Storage: Name=%s Used=%d Total=%d", storage.Name, storage.Used, storage.Total)

	// 6. USSD
	ussdResp, err := driver.SendUSSD("*100#")
	if err != nil {
		t.Fatalf("driver.SendUSSD failed: %v", err)
	}
	t.Logf("USSD immediate response: %q", ussdResp)

	// Wait for async +CUSD URC
	select {
	case urc := <-engine.URC():
		t.Logf("Received URC: %s", urc)
	case <-time.After(10 * time.Second):
		t.Log("No URC received within 10s (network might be slow)")
	}
}

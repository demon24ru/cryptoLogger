package connector

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"io"
	"net"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
)

// Websocket is for websocket connection.
type Websocket struct {
	Conn net.Conn
	Cfg  *config.WS
}

// NewWebsocket creates a new websocket connection for the exchange.
func NewWebsocket(appCtx context.Context, cfg *config.WS, url string) (Websocket, error) {
	var ctx context.Context
	if cfg.ConnTimeoutSec > 0 {
		timeoutCtx, cancel := context.WithTimeout(appCtx, time.Duration(cfg.ConnTimeoutSec)*time.Second)
		ctx = timeoutCtx
		defer cancel()
	} else {
		ctx = context.Background()
	}
	conn, _, _, err := ws.Dial(ctx, url)
	if err != nil {
		return Websocket{}, err
	}
	websocket := Websocket{Conn: conn, Cfg: cfg}
	return websocket, nil
}

// Write writes data frame on websocket connection.
func (w *Websocket) Write(data []byte) error {
	err := wsutil.WriteClientText(w.Conn, data)
	if err != nil {
		return err
	}
	return nil
}

// Read reads data frame from websocket connection.
func (w *Websocket) Read() ([]byte, error) {
	if w.Cfg.ReadTimeoutSec > 0 {
		err := w.Conn.SetReadDeadline(time.Now().Add(time.Duration(w.Cfg.ReadTimeoutSec) * time.Second))
		if err != nil {
			return nil, err
		}
	}
	data, _, err := wsutil.ReadServerData(w.Conn)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ReadTextOrGzipBinary reads data frame from websocket connection.
// It also handles gzip compressed binary data frame.
func (w *Websocket) ReadTextOrGzipBinary() ([]byte, error) {
	if w.Cfg.ReadTimeoutSec > 0 {
		err := w.Conn.SetReadDeadline(time.Now().Add(time.Duration(w.Cfg.ReadTimeoutSec) * time.Second))
		if err != nil {
			return nil, err
		}
	}
	data, dataType, err := wsutil.ReadServerData(w.Conn)
	if err != nil {
		return nil, err
	}
	if dataType != ws.OpBinary {
		return data, nil
	}

	// If the server sends compressed binary data, then we need to decompress it.
	buf := bytes.NewBuffer(data)
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ReadTextOrFlateBinary reads data frame from websocket connection.
// It also handles flate compressed binary data frame.
func (w *Websocket) ReadTextOrFlateBinary() ([]byte, error) {
	if w.Cfg.ReadTimeoutSec > 0 {
		err := w.Conn.SetReadDeadline(time.Now().Add(time.Duration(w.Cfg.ReadTimeoutSec) * time.Second))
		if err != nil {
			return nil, err
		}
	}
	data, dataType, err := wsutil.ReadServerData(w.Conn)
	if err != nil {
		return nil, err
	}
	if dataType != ws.OpBinary {
		return data, nil
	}

	// If the server sends compressed binary data, then we need to decompress it.
	buf := bytes.NewBuffer(data)
	reader := flate.NewReader(buf)
	defer reader.Close()
	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ReadTextOrZlibBinary reads data frame from websocket connection.
// It also handles zlib compressed binary data frame.
func (w *Websocket) ReadTextOrZlibBinary() ([]byte, error) {
	if w.Cfg.ReadTimeoutSec > 0 {
		err := w.Conn.SetReadDeadline(time.Now().Add(time.Duration(w.Cfg.ReadTimeoutSec) * time.Second))
		if err != nil {
			return nil, err
		}
	}
	data, dataType, err := wsutil.ReadServerData(w.Conn)
	if err != nil {
		return nil, err
	}
	if dataType != ws.OpBinary {
		return data, nil
	}

	// If the server sends compressed binary data, then we need to decompress it.
	buf := bytes.NewBuffer(data)
	reader, err := zlib.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Ping writes ping frame on websocket connection.
// (Websocket standard PING frame, not the application level one)
func (w *Websocket) Ping() error {
	err := wsutil.WriteClientMessage(w.Conn, ws.OpPing, []byte{})
	if err != nil {
		return err
	}
	return nil
}

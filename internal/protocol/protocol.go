package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	Magic         = "PTNL"
	Version uint8 = 1

	MessageOpen      uint8 = 1
	MessageOpenOK    uint8 = 2
	MessageOpenError uint8 = 3
)

const (
	maxPayloadSize = 4096
)

var (
	ErrInvalidMagic   = errors.New("invalid protocol magic")
	ErrInvalidVersion = errors.New("unsupported protocol version")
	ErrInvalidMessage = errors.New("invalid protocol message")
)

type Header struct {
	Version uint8
	Type    uint8
	Length  uint16
}

func WriteOpen(w io.Writer, listenerID string) error {
	if len(listenerID) == 0 {
		return fmt.Errorf("listener ID is empty")
	}

	if len(listenerID) > maxPayloadSize {
		return fmt.Errorf("listener ID is too long")
	}

	return writeMessage(w, MessageOpen, []byte(listenerID))
}

func ReadOpen(r io.Reader) (string, error) {
	msgType, payload, err := readMessage(r)
	if err != nil {
		return "", err
	}

	if msgType != MessageOpen {
		return "", fmt.Errorf("%w: expected OPEN, got %d", ErrInvalidMessage, msgType)
	}

	if len(payload) == 0 {
		return "", fmt.Errorf("%w: empty listener ID", ErrInvalidMessage)
	}

	return string(payload), nil
}

func WriteOpenOK(w io.Writer) error {
	return writeMessage(w, MessageOpenOK, nil)
}

func WriteOpenError(w io.Writer, message string) error {
	if len(message) > maxPayloadSize {
		message = message[:maxPayloadSize]
	}

	return writeMessage(w, MessageOpenError, []byte(message))
}

func ReadOpenResponse(r io.Reader) (ok bool, remoteError string, err error) {
	msgType, payload, err := readMessage(r)
	if err != nil {
		return false, "", err
	}

	switch msgType {
	case MessageOpenOK:
		return true, "", nil

	case MessageOpenError:
		return false, string(payload), nil

	default:
		return false, "", fmt.Errorf(
			"%w: expected OPEN_OK or OPEN_ERROR, got %d",
			ErrInvalidMessage,
			msgType,
		)
	}
}

func writeMessage(w io.Writer, msgType uint8, payload []byte) error {
	if len(payload) > maxPayloadSize {
		return fmt.Errorf("payload too large: %d", len(payload))
	}

	var header [8]byte

	copy(header[0:4], Magic)

	header[4] = Version
	header[5] = msgType

	binary.BigEndian.PutUint16(header[6:8], uint16(len(payload)))

	if _, err := w.Write(header[:]); err != nil {
		return fmt.Errorf("write protocol header: %w", err)
	}

	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return fmt.Errorf("write protocol payload: %w", err)
		}
	}

	return nil
}

func readMessage(r io.Reader) (uint8, []byte, error) {
	var header [8]byte

	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, fmt.Errorf("read protocol header: %w", err)
	}

	if string(header[0:4]) != Magic {
		return 0, nil, ErrInvalidMagic
	}

	if header[4] != Version {
		return 0, nil, fmt.Errorf(
			"%w: %d",
			ErrInvalidVersion,
			header[4],
		)
	}

	length := binary.BigEndian.Uint16(header[6:8])

	if length > maxPayloadSize {
		return 0, nil, fmt.Errorf(
			"protocol payload too large: %d",
			length,
		)
	}

	payload := make([]byte, length)

	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, fmt.Errorf("read protocol payload: %w", err)
		}
	}

	return header[5], payload, nil
}
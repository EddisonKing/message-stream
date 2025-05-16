package messagestream

import (
	"io"
)

func writeV(data []byte, w io.Writer) error {
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

func writeLV(data []byte, w io.Writer) error {
	dataSizeBytes := intToBytes(len(data))

	if _, err := w.Write(dataSizeBytes); err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return err
	}

	return nil
}

func readV(size int, r io.Reader) ([]byte, error) {
	data := make([]byte, size)
	if _, err := r.Read(data); err != nil {
		return nil, err
	}

	return data, nil
}

func readLV(size int, r io.Reader) ([]byte, error) {
	sizeBytes := make([]byte, size)
	if _, err := r.Read(sizeBytes); err != nil {
		return nil, err
	}

	valueSize := bytesToInt(sizeBytes)
	if valueSize <= 0 {
		return []byte{}, nil
	}

	data := make([]byte, valueSize)
	if _, err := r.Read(data); err != nil {
		return nil, err
	}

	return data, nil
}

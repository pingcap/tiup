package store

import (
	"io"
	"os"
)

// Store represents the storage level
type Store interface {
	Begin() (FsTxn, error)
}

// FsTxn represent the transaction session of file operations
type FsTxn interface {
	Write(filename string, reader io.Reader) error
	Read(filename string) (io.ReadCloser, error)
	WriteManifest(filename string, manifest interface{}) error
	ReadManifest(filename string, manifest interface{}) error
	Stat(filename string) (os.FileInfo, error)
	// Restart should reset the manifest state
	ResetManifest() error
	Commit() error
	Rollback() error
}

// NewStore returns a Store, curretly only qcloud supported
func NewStore(root string) Store {
	return newQCloudStore(root)
}

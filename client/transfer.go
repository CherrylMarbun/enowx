package client

import (
	"io"

	"github.com/enowx/enowx/proto"
)

// PackTransfer packs a project and its sessions into an archive on the
// daemon's host, to carry to another host.
func (c *Client) PackTransfer(p proto.TransferPackParams) (proto.TransferInfo, error) {
	var info proto.TransferInfo
	err := c.Call(proto.MethodTransferPack, p, &info)
	return info, err
}

// ReadTransfer reads a part of archive id, from offset.
func (c *Client) ReadTransfer(id string, offset int64) ([]byte, error) {
	var part proto.TransferPart
	err := c.Call(proto.MethodTransferRead, proto.TransferReadParams{ID: id, Offset: offset, Size: proto.TransferPartSize}, &part)
	return part.Data, err
}

// WriteTransfer writes a part of archive id, at offset; 0 starts it.
func (c *Client) WriteTransfer(id string, offset int64, data []byte) error {
	return c.Call(proto.MethodTransferWrite, proto.TransferPart{ID: id, Offset: offset, Data: data}, nil)
}

// UnpackTransfer unpacks archive id into the folder dest, and starts its
// sessions again.
func (c *Client) UnpackTransfer(id, dest string) (proto.TransferResult, error) {
	var res proto.TransferResult
	err := c.Call(proto.MethodTransferUnpack, proto.TransferUnpackParams{ID: id, Dest: dest}, &res)
	return res, err
}

// DiscardTransfer deletes archive id.
func (c *Client) DiscardTransfer(id string) error {
	return c.Call(proto.MethodTransferDiscard, proto.TransferRef{ID: id}, nil)
}

// CopyTransfer carries archive id, of size bytes, from one daemon to
// another, part by part; progress learns how much has gone.
func CopyTransfer(from, to *Client, id string, size int64, progress func(done int64)) error {
	var done int64
	for done < size || done == 0 {
		data, err := from.ReadTransfer(id, done)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			if done < size {
				return io.ErrUnexpectedEOF
			}
			break
		}
		if err := to.WriteTransfer(id, done, data); err != nil {
			return err
		}
		done += int64(len(data))
		if progress != nil {
			progress(done)
		}
	}
	return nil
}

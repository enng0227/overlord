package redis

import (
	errs "errors"
	"sync/atomic"
	"time"

	"overlord/lib/bufio"
	libnet "overlord/lib/net"
	"overlord/proto"

	"github.com/pkg/errors"
)

const (
	opened = int32(0)
	closed = int32(1)
)

var (
	ErrNodeConnClosed = errs.New("redis node conn closed")
)

type nodeConn struct {
	cluster string
	addr    string
	conn    *libnet.Conn
	bw      *bufio.Writer
	br      *bufio.Reader

	state int32
}

// NewNodeConn create the node conn from proxy to redis
func NewNodeConn(cluster, addr string, dialTimeout, readTimeout, writeTimeout time.Duration) (nc proto.NodeConn) {
	conn := libnet.DialWithTimeout(addr, dialTimeout, readTimeout, writeTimeout)
	return newNodeConn(cluster, addr, conn)
}

func newNodeConn(cluster, addr string, conn *libnet.Conn) proto.NodeConn {
	return &nodeConn{
		cluster: cluster,
		addr:    addr,
		conn:    conn,
		br:      bufio.NewReader(conn, nil),
		bw:      bufio.NewWriter(conn),
	}
}

func (nc *nodeConn) WriteBatch(mb *proto.MsgBatch) (err error) {
	if nc.Closed() {
		err = errors.Wrap(ErrNodeConnClosed, "Redis Reader read batch message")
		return
	}
	for _, m := range mb.Msgs() {
		req, ok := m.Request().(*Request)
		if !ok {
			m.WithError(ErrBadAssert)
			return ErrBadAssert
		}
		if !req.isSupport() || req.isCtl() {
			continue
		}
		if err = req.resp.encode(nc.bw); err != nil {
			m.WithError(err)
			return err
		}
		m.MarkWrite()
	}
	return nc.bw.Flush()
}

func (nc *nodeConn) ReadBatch(mb *proto.MsgBatch) (err error) {
	if nc.Closed() {
		err = errors.Wrap(ErrNodeConnClosed, "Redis Reader read batch message")
		return
	}
	nc.br.ResetBuffer(mb.Buffer())
	defer nc.br.ResetBuffer(nil)
	begin := nc.br.Mark()
	now := nc.br.Mark()
	for i := 0; i < mb.Count(); {
		m := mb.Nth(i)
		req, ok := m.Request().(*Request)
		if !ok {
			return ErrBadAssert
		}
		if !req.isSupport() || req.isCtl() {
			i++
			continue
		}
		if err = req.reply.Decode(nc.br); err == bufio.ErrBufferFull {
			nc.br.AdvanceTo(begin)
			if err = nc.br.Read(); err != nil {
				return
			}
			nc.br.AdvanceTo(now)
			continue
		} else if err != nil {
			return
		}
		m.MarkRead()
		now = nc.br.Mark()
		i++
	}
	return
}

func (nc *nodeConn) Close() (err error) {
	if atomic.CompareAndSwapInt32(&nc.state, opened, closed) {
		return nc.conn.Close()
	}
	return
}

func (nc *nodeConn) Closed() bool {
	return atomic.LoadInt32(&nc.state) == closed
}

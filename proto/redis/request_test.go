package redis

import (
	"testing"

	"overlord/lib/bufio"

	"github.com/stretchr/testify/assert"
)

func TestRequestNewRequest(t *testing.T) {
	var bs = []byte("*2\r\n$4\r\nLLEN\r\n$6\r\nmylist\r\n")
	// conn
	conn := _createConn(bs)
	br := bufio.NewReader(conn, bufio.Get(1024))
	br.Read()
	req := getReq()
	err := req.resp.Decode(br)
	assert.Nil(t, err)
	assert.Equal(t, mergeTypeNo, req.mType)
	assert.Equal(t, 2, req.resp.arrayn)
	assert.Equal(t, "LLEN", req.CmdString())
	assert.Equal(t, []byte("LLEN"), req.Cmd())
	assert.Equal(t, "mylist", string(req.Key()))
}

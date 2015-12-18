package gncp

import (
	"net"
)

type CpConn struct {
	net.Conn
	pool   *GncpPool
	inpool bool
}

// Destroy will close connection and release connection from connection pool.
func (conn *CpConn) Destroy() error {
	err := conn.pool.Remove(conn.Conn)
	if err != nil {
		return err
	}
	return conn.Conn.Close()
}

// Close will push connection back to connection pool. It will not close the real connection.
func (conn *CpConn) Close() error {
	return conn.pool.Put(conn.Conn)
}

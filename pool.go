package gncp

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type ConnPool interface {
	Get() (net.Conn, error)
	GetWithTimeout(timeout time.Duration) (net.Conn, error)
	Close() error
	Remove(conn net.Conn) error
}

type GncpPool struct {
	lock         sync.Mutex
	conns        chan net.Conn
	minConnNum   int
	maxConnNum   int
	totalConnNum int
	closed       bool
	connCreator  func() (net.Conn, error)
}

var PoolIsCloseError = errors.New("Connection pool has been closed.")

func NewPool(minConn, maxConn int, connCreator func() (net.Conn, error)) (*GncpPool, error) {
	if minConn > maxConn || minConn < 0 || maxConn <= 0 {
		return nil, errors.New("Number of connection bound error.")
	}

	pool := &GncpPool{}
	pool.minConnNum = minConn
	pool.maxConnNum = maxConn
	pool.connCreator = connCreator
	pool.conns = make(chan net.Conn, maxConn)
	pool.closed = false
	pool.totalConnNum = 0
	err := pool.init()
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func (p *GncpPool) init() error {
	for i := 0; i < p.minConnNum; i++ {
		conn, err := p.createConn()
		if err != nil {
			return err
		}
		p.conns <- conn
	}
	return nil
}

func (p *GncpPool) Get() (net.Conn, error) {
	if p.isClosed() == true {
		return nil, PoolIsCloseError
	}
	go func() {
		conn, err := p.createConn()
		if err != nil {
			return
		}
		p.conns <- conn
	}()
	select {
	case conn := <-p.conns:
		return p.packConn(conn), nil
	}
}

func (p *GncpPool) GetWithTimeout(timeout time.Duration) (net.Conn, error) {
	if p.isClosed() == true {
		return nil, PoolIsCloseError
	}
	go func() {
		conn, err := p.createConn()
		if err != nil {
			return
		}
		p.conns <- conn
	}()
	select {
	case conn := <-p.conns:
		return p.packConn(conn), nil
	case <-time.After(timeout):
		return nil, errors.New("Get Connection timeout.")
	}
}

func (p *GncpPool) Close() error {
	if p.isClosed() == true {
		return PoolIsCloseError
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.closed = true
	close(p.conns)
	for conn := range p.conns {
		conn.Close()
	}
	return nil
}

func (p *GncpPool) Put(conn net.Conn) error {
	if p.isClosed() == true {
		return PoolIsCloseError
	}
	if conn == nil {
		p.lock.Lock()
		p.totalConnNum = p.totalConnNum - 1
		p.lock.Unlock()
		return errors.New("Cannot put nil to connection pool.")
	}

	select {
	case p.conns <- conn:
		return nil
	default:
		return conn.Close()
	}
}

func (p *GncpPool) isClosed() bool {
	p.lock.Lock()
	ret := p.closed
	p.lock.Unlock()
	return ret
}

// RemoveConn let conn not belong connection pool.
func (p *GncpPool) Remove(conn net.Conn) error {
	if p.isClosed() == true {
		return PoolIsCloseError
	}
	p.lock.Lock()
	p.totalConnNum = p.totalConnNum - 1
	p.lock.Lock()
	return nil
}

// createConn will create one connection from connCreator. And increase connection counter.
func (p *GncpPool) createConn() (net.Conn, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.totalConnNum >= p.maxConnNum {
		return nil, fmt.Errorf("Connot Create new connection. Now has %d.Max is %d", p.totalConnNum, p.maxConnNum)
	}
	conn, err := p.connCreator()
	if err != nil {
		return nil, fmt.Errorf("Cannot create new connection.%s", err)
	}
	p.totalConnNum = p.totalConnNum + 1
	return conn, nil
}

func (p *GncpPool) packConn(conn net.Conn) net.Conn {
	ret := &CpConn{pool: p}
	ret.inpool = true
	ret.Conn = conn
	return ret
}

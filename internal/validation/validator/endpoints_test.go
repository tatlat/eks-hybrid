package validator_test

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/aws/eks-hybrid/internal/validation/util/mocks"
	"github.com/aws/eks-hybrid/internal/validation/validator"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
)

func TestEndpointsValidateFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := NewWithT(t)
	client := mocks.NewMockNetClient(ctrl)
	r := validator.NewRunner()
	n := validator.NewEndpoints("", client)
	r.Register(n)

	client.EXPECT().DialTimeout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("fake fail")).Times(16)
	g.Expect(r.Run()).ToNot(Succeed())
	err := r.Run()
	if err != nil {
		if customErr, ok := err.(*validator.FailError); ok {
			t.Errorf("Received error: %v", customErr)
		}
	}
}

func TestEndpointsValidateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := NewWithT(t)
	conn := NewMockConn(ctrl)
	conn.EXPECT().Close().Return(nil).Times(8)
	client := mocks.NewMockNetClient(ctrl)
	r := validator.NewRunner()
	n := validator.NewEndpoints("", client)
	r.Register(n)

	client.EXPECT().DialTimeout(gomock.Any(), gomock.Any(), gomock.Any()).Return(conn, nil).Times(8)
	g.Expect(r.Run()).To(Succeed())
}

// MockConn is a mock of NetClient interface. It is hand written.
type MockConn struct {
	ctrl     *gomock.Controller
	recorder *MockConnMockRecorder
}

var _ net.Conn = &MockConn{}

// MockConnMockRecorder is the mock recorder for MockConn.
type MockConnMockRecorder struct {
	mock *MockConn
}

// NewMockConn creates a new mock instance.
func NewMockConn(ctrl *gomock.Controller) *MockConn {
	mock := &MockConn{ctrl: ctrl}
	mock.recorder = &MockConnMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConn) EXPECT() *MockConnMockRecorder {
	return m.recorder
}

// DialTimeout mocks base method.
func (m *MockConn) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

func (m *MockConn) Read(b []byte) (n int, err error)   { panic("unimplemented") }
func (m *MockConn) Write(b []byte) (n int, err error)  { panic("unimplemented") }
func (m *MockConn) LocalAddr() net.Addr                { panic("unimplemented") }
func (m *MockConn) RemoteAddr() net.Addr               { panic("unimplemented") }
func (m *MockConn) SetDeadline(t time.Time) error      { panic("unimplemented") }
func (m *MockConn) SetReadDeadline(t time.Time) error  { panic("unimplemented") }
func (m *MockConn) SetWriteDeadline(t time.Time) error { panic("unimplemented") }

func (mr *MockConnMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockConn)(nil).Close))
}

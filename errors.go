package ta

import "fmt"

// TALibError is the panic value for all TA-Lib failures.
// Callers can recover() and type-assert to *TALibError to inspect the code.
type TALibError struct {
	RetCode int
	Message string
}

func (e *TALibError) Error() string { return e.Message }

func retCodeMessage(code int) string {
	if msg, ok := retCodeMessages[code]; ok {
		return msg
	}
	return fmt.Sprintf("unknown TA-Lib error code %d", code)
}

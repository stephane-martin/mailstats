package utils

func IsTimeout(e error) bool {
	if err, ok := e.(iTimeout); ok {
		return err.Timeout()
	}
	return false
}


func IsTemp(e error) bool {
	if err, ok := e.(iTemporary); ok {
		return err.Temporary()
	}
	return false
}

type iTimeout interface {
	Timeout() bool
}

type iTemporary interface {
	Temporary() bool
}



package extractors

var v struct{}

var exeTypes = map[string]struct{}{
	"application/x-dosexec": v,
	"application/x-msdownload": v,
	"application/exe": v,
	"application/x-exe": v,
	"application/dos-exe": v,
	"vms/exe": v,
	"application/x-winexe": v,
	"application/msdos-windows": v,
	"application/x-msdos-program": v,
	"application/x-executable": v,
	"application/vnd.microsoft.portable-executable": v,
}

func IsExecutable(mimetype string) bool {
	_, ok := exeTypes[mimetype]
	return ok
}

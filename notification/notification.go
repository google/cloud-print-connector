package notification

type PrinterNotificationType uint8
type PrinterNotification struct {
	GCPID string
	Type  PrinterNotificationType
}

const (
	PrinterNewJobs PrinterNotificationType = iota
	PrinterDelete
)

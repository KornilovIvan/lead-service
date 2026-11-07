package service

const (
	crmLabelSynced  = "в CRM ушло"
	crmLabelFailed  = "выгрузка упала"
	crmLabelNotSync = "в CRM ещё нет"
)

// CRMLabel describes the export state using only values already loaded from
// the database. It never opens a DB connection.
func CRMLabel(bitrixID *string, latestJobStatus *string) string {
	if bitrixID != nil && *bitrixID != "" {
		return crmLabelSynced
	}
	if latestJobStatus != nil && *latestJobStatus == "failed" {
		return crmLabelFailed
	}
	return crmLabelNotSync
}

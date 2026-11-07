package service

import "testing"

func labelPtr(v string) *string { return &v }

func TestCRMLabel(t *testing.T) {
	bitrixID := "123"
	failed := "failed"
	queued := "queued"
	inProgress := "in_progress"
	done := "done"

	tests := []struct {
		name     string
		bitrixID *string
		latest   *string
		want     string
	}{
		{"bitrix id and failed", &bitrixID, &failed, "в CRM ушло"},
		{"bitrix id and queued", &bitrixID, &queued, "в CRM ушло"},
		{"bitrix id without jobs", &bitrixID, nil, "в CRM ушло"},
		{"no bitrix id and failed", nil, &failed, "выгрузка упала"},
		{"no bitrix id and queued", nil, &queued, "в CRM ещё нет"},
		{"no bitrix id and in_progress", nil, &inProgress, "в CRM ещё нет"},
		{"no bitrix id and no jobs", nil, nil, "в CRM ещё нет"},
		{"no bitrix id and done", nil, &done, "в CRM ещё нет"},
		{"empty bitrix id and failed", labelPtr(""), &failed, "выгрузка упала"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CRMLabel(tt.bitrixID, tt.latest); got != tt.want {
				t.Fatalf("CRMLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

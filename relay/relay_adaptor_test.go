package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	taskxai "github.com/QuantumNous/new-api/relay/channel/task/xai"
)

func TestGetTaskAdaptorXai(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform("48"))
	if adaptor == nil {
		t.Fatal("GetTaskAdaptor(ChannelTypeXai) = nil")
	}
	if _, ok := adaptor.(*taskxai.TaskAdaptor); !ok {
		t.Fatalf("GetTaskAdaptor(ChannelTypeXai) = %T, want *xai.TaskAdaptor", adaptor)
	}
}

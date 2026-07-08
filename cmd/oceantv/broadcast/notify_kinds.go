package broadcast

import "github.com/ausocean/cloud/notify"

const (
	KindGeneric       notify.Kind = "broadcast-generic"       // Problems where cause is unknown or un-categorized.
	KindForwarder     notify.Kind = "broadcast-forwarder"     // Problems related to our forwarding service i.e. can't stream slate.
	KindHardware      notify.Kind = "broadcast-hardware"      // Problems related to streaming hardware i.e. controllers and cameras.
	KindNetwork       notify.Kind = "broadcast-network"       // Problems related to bad bandwidth, generally indicated by bad health events.
	KindSoftware      notify.Kind = "broadcast-software"      // Problems related to the functioning of our broadcast software.
	KindConfiguration notify.Kind = "broadcast-configuration" // Problems related to the configuration of the broadcast.
	KindService       notify.Kind = "broadcast-service"       // Problems related to the broadcast service e.g. YouTube API issues.
)

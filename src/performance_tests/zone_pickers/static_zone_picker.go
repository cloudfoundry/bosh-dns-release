package zone_pickers

type StaticZonePicker struct {
	zone string
}

func NewStaticZonePicker(zone string) *StaticZonePicker {
	return &StaticZonePicker{zone: zone}
}

func (picker *StaticZonePicker) NextZone() string {
	return picker.zone
}

package performance_test

import "time"

func healthVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 40,
		MemPct99: 22,
		MemMax:   30,
	}
}

func healthTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   12 * time.Millisecond,
		Pct90: 14 * time.Millisecond,
		Pct95: 15 * time.Millisecond,
	}
}

func prodLikeVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 5,
		MemPct99: 21,
		MemMax:   23,
	}
}

func localZonesVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 10,
		MemPct99: 38,
		MemMax:   40,
	}
}

func localZonesTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   3 * time.Millisecond,
		Pct90: 4 * time.Millisecond,
		Pct95: 5 * time.Millisecond,
	}
}

func upcheckVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 1,
		MemPct99: 23,
		MemMax:   25,
	}
}

func upcheckTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   1 * time.Millisecond,
		Pct90: 2 * time.Millisecond,
		Pct95: 2 * time.Millisecond,
	}
}

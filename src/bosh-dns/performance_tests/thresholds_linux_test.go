package performance_test

import "time"

func healthVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 50,
		MemPct99: 24,
		MemMax:   30,
	}
}

func healthTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   14 * time.Millisecond,
		Pct90: 18 * time.Millisecond,
		Pct95: 20 * time.Millisecond,
	}
}

func prodLikeVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 5,
		MemPct99: 23,
		MemMax:   24,
	}
}

func localZonesVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 25,
		MemPct99: 45,
		MemMax:   50,
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
		CPUPct99: 3,
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

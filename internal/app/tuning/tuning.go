// package tuning holds runtime-adjustable performance knobs for the upload
// pipelines (animation/sound) the desktop app posts detected pc specs to
// /tuning at launch so we can scale concurrency and request rate up or down
// instead of hardcoding values that are conservative on a workstation and
// too aggressive on a low-end laptop
package tuning

import "sync/atomic"

const (
	defaultAnimationStartsPerMinute = 420
	defaultAnimationMaxConcurrent   = 24
	defaultSoundUploadsPerMinute    = 120
)

var (
	animationStartsPerMinute atomic.Int32
	animationMaxConcurrent   atomic.Int32
	soundUploadsPerMinute    atomic.Int32
)

func init() {
	animationStartsPerMinute.Store(defaultAnimationStartsPerMinute)
	animationMaxConcurrent.Store(defaultAnimationMaxConcurrent)
	soundUploadsPerMinute.Store(defaultSoundUploadsPerMinute)
}

func AnimationStartsPerMinute() int { return int(animationStartsPerMinute.Load()) }
func AnimationMaxConcurrent() int   { return int(animationMaxConcurrent.Load()) }
func SoundUploadsPerMinute() int    { return int(soundUploadsPerMinute.Load()) }

// apply updates any knob with a value > 0 zero/negative inputs are ignored so
// the desktop app can partially override fields without resetting the rest
func Apply(animStartsPerMinute, animMaxConcurrent, soundUploadsPerMin int) {
	if animStartsPerMinute > 0 {
		animationStartsPerMinute.Store(int32(animStartsPerMinute))
	}
	if animMaxConcurrent > 0 {
		animationMaxConcurrent.Store(int32(animMaxConcurrent))
	}
	if soundUploadsPerMin > 0 {
		soundUploadsPerMinute.Store(int32(soundUploadsPerMin))
	}
}

// snapshot returns the current values as a map suitable for json encoding
// used by the /tuning get so the desktop app can confirm what the backend is
// actually running with after sending an apply
func Snapshot() map[string]int {
	return map[string]int{
		"animationStartsPerMinute": AnimationStartsPerMinute(),
		"animationMaxConcurrent":   AnimationMaxConcurrent(),
		"soundUploadsPerMinute":    SoundUploadsPerMinute(),
	}
}

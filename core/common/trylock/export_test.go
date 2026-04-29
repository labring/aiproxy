package trylock

func InjectMemLockValueForTest(key string, value any) {
	memRecord.Store(key, value)
}

func ResetMemLockValueForTest(key string) {
	memRecord.Delete(key)
}

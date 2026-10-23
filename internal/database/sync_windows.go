package database

// File.Sync flushes the snapshot itself. Directory fsync is not portable to
// Windows; installation/power-loss behavior still requires target-OS testing.
func syncDirectory(string) error { return nil }

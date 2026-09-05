package storage

import "testing"

func TestDirectoryLockExcludesOtherOwners(t *testing.T) {
	dir := t.TempDir()
	unlock, err := LockDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := LockDirectory(dir); err == nil {
		other()
		t.Fatal("two directory owners")
	}
	unlock()
	unlock, err = LockDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

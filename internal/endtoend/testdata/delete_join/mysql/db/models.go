// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.18.0

package db

import ()

type JoinTable struct {
	ID             uint64
	PrimaryTableID uint64
	OtherTableID   uint64
	IsActive       bool
}

type PrimaryTable struct {
	ID     uint64
	UserID uint64
}
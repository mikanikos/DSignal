package helpers

import (
	//"fmt"
	"net"
	"sort"
)

// BaseAddress of the program, local address as default
const BaseAddress = "127.0.0.1"

// Message struct
type Message struct {
	Text        string
	Destination *string
	File        *string
	Request     *[]byte
	Keywords    *string
	Budget      *uint64
	Identity    *string
}

// ErrorCheck to log errors
func ErrorCheck(err error, doPanic bool) {
	if err != nil {
		if doPanic {
			panic(err)
		} else {
			//fmt.Println("Error ", err)
		}
	}
}

// DifferenceString to do the difference between two string sets
func DifferenceString(list1, list2 []*net.UDPAddr) []*net.UDPAddr {
	mapList2 := make(map[string]struct{}, len(list2))
	for _, x := range list2 {
		mapList2[x.String()] = struct{}{}
	}
	var difference []*net.UDPAddr
	for _, x := range list1 {
		_, check := mapList2[x.String()]
		if !check {
			difference = append(difference, x)
		}
	}
	return difference
}

// GetArrayStringFromAddresses to convert array of addresses to array of strings
func GetArrayStringFromAddresses(peers []*net.UDPAddr) []string {
	list := make([]string, 0)
	for _, p := range peers {
		list = append(list, p.String())
	}
	return list
}

// RemoveDuplicatesFromStringSlice utility to remove duplicates from list of strings
func RemoveDuplicatesFromStringSlice(slice []string) []string {
	found := make(map[string]bool)
	for i := range slice {
		found[slice[i]] = true
	}

	result := []string{}
	for key := range found {
		result = append(result, key)
	}
	return result
}

// SortUint64 utility to sort a slice of uint64
func SortUint64(slice []uint64) {
	sort.Slice(slice, func(i, j int) bool { return slice[i] < slice[j] })
}

// InsertToSortUint64Slice utility to insert uint64 in sorted slice
func InsertToSortUint64Slice(data []uint64, el uint64) []uint64 {
	index := sort.Search(len(data), func(i int) bool { return data[i] > el })
	data = append(data, 0)
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}

// RemoveDuplicatesFromUint64Slice utility to remove duplicates from list of strings
func RemoveDuplicatesFromUint64Slice(slice []uint64) []uint64 {
	found := make(map[uint64]bool)
	for i := range slice {
		found[slice[i]] = true
	}

	result := []uint64{}
	for key := range found {
		result = append(result, key)
	}
	return result
}

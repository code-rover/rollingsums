package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"math/rand"
	"time"
	"unsafe"

	"github.com/pkg/profile"
)

type Patch struct {
	data []byte
	pos  int
	len  int
}

var step = 500

func RebuildFile(origin []byte, patchList []*Patch) *bytes.Buffer {
	var buf bytes.Buffer
	for _, patch := range patchList {
		if patch.pos == -1 {
			buf.Write(patch.data)
			// println("data: " + string(patch.data))
		} else {
			// println("patch: " + string(origin[patch.pos:patch.pos+patch.len]))
			buf.Write(origin[patch.pos : patch.pos+patch.len])
		}
	}
	return &buf
}

func Alder32Sum(data []byte) uint32 {
	a := 1
	b := 0
	for i := 0; i < len(data); i++ {
		a += int(data[i])
		b += a
	}
	a %= 65521
	b %= 65521
	return uint32(b<<16 | a&0xffff)
}

//根据之前结果增量计算
func Alder32SumBasedOnPrev(data []byte, curPos int, prev uint32) uint32 {
	d1 := uint32(data[curPos-step])
	d2 := uint32(data[curPos])
	prevA := prev & 0xffff
	prevB := (prev >> 16) & 0xffff
	prevA -= d1
	prevA += d2
	prevB -= uint32(step) * d1
	prevB--
	prevB += prevA
	prevA %= 65521
	prevB %= 65521
	return prevB<<16 | prevA&0xffff
}

func MakePatch(f2 []byte, sumList *SumList) []*Patch {
	blockMap := make(map[uint32][]*SumPos, len(sumList.list))
	for i := 0; i < len(sumList.list); i++ {
		blockMap[sumList.list[i].sum1] = sumList.list[i].sum2
	}

	dataLen := len(f2)

	patchList := make([]*Patch, 0, len(sumList.list))
	var backItem *Patch

	var bufA = -1 //差异开始段位置
	var bufB = -1 //差异结束段位置
	i := 0

	var sum1 uint32

	for i = 0; i+step <= dataLen; {
		backItem = nil
		if len(patchList) > 0 {
			backItem = patchList[len(patchList)-1]
		}

		if bufA != -1 && i > step && sum1 > 0 {
			sum1 = Alder32SumBasedOnPrev(f2, i, sum1) //根据上一结果增量计算,bufA不等于-1意味着上一步是连续差异数据，可以借用上次结果增量计算本次alder32值
		} else {
			sum1 = Alder32Sum(f2[i : i+step])
		}

		sum2List, isSum1Exist := blockMap[sum1]
		sumPos := -1
		if isSum1Exist { //需要继续检查sum2
			for _, sum2Pos := range sum2List {
				if sum2Pos.sum2 == md5sum(f2[i:i+step]) {
					sumPos = sum2Pos.pos
					// fmt.Printf("sumPos: %d  str: %s\n", sumPos, f2[i:i+step])
					break
				}
			}
		}

		if isSum1Exist && sumPos > -1 {
			if bufA != -1 {
				buf := bytes.NewBuffer(backItem.data)
				buf.Write(f2[bufA:bufB])
				backItem.data = buf.Bytes()
				bufA = -1
				bufB = -1
			}
			// fmt.Printf("find: %d   %d   %s\n", sum1, sumPos, f2[i:i+step])

			//优化 队列上一个元素不是字符串 或  间断块
			if backItem == nil || backItem.pos == -1 || sumPos != backItem.pos+backItem.len {
				backItem = &Patch{
					pos: sumPos,
					len: step,
				}
				patchList = append(patchList, backItem)
			} else {
				backItem.len += step
			}

			i += step

		} else { //差异部分
			if backItem == nil || backItem.pos > -1 {
				backItem = &Patch{
					pos: -1,
				}
				patchList = append(patchList, backItem)
			}

			if bufA == -1 {
				bufA = i
				bufB = i
			}
			bufB++
			i++
		}
		//println(i)
	}

	//剩余差异内容
	if bufA > -1 {
		buf := bytes.NewBuffer(backItem.data)
		buf.Write(f2[bufA:bufB])
		backItem.data = buf.Bytes()
	}

	//剩余block处理
	if i+step > dataLen { //不足一个block的剩余
		if backItem == nil || backItem.pos > -1 {
			backItem = &Patch{pos: -1}
			patchList = append(patchList, backItem)
		}
		buf := bytes.NewBuffer(backItem.data)
		buf.Write(f2[i:len(f2)])
		backItem.data = buf.Bytes()
	}

	return patchList
}

type SumPos struct {
	sum2 string
	pos  int
}

type SumInfo struct {
	sum1 uint32
	sum2 []*SumPos
}

type SumList struct {
	list []*SumInfo
}

func MakeSumList(data []byte) *SumList {
	var sumList SumList
	if len(data) < step {
		return &sumList
	}

	sumMap := make(map[uint32]*SumInfo)

	for i := 0; i <= len(data)-step; i += step {
		sum1 := Alder32Sum(data[i : i+step])
		sum2 := md5sum(data[i : i+step])

		if _, ok := sumMap[sum1]; !ok {
			sumMap[sum1] = &SumInfo{
				sum1: sum1,
				sum2: []*SumPos{{sum2: sum2, pos: i}},
			}
		} else {
			sumMap[sum1].sum2 = append(sumMap[sum1].sum2, &SumPos{sum2, i})
		}
	}

	for _, v := range sumMap {
		sumList.list = append(sumList.list, v)
	}

	return &sumList
}

func Diff(f1 []byte, f2 []byte) bool {

	//t := time.Now()
	sumList := MakeSumList(f1)
	//elapsed := time.Since(t)
	//fmt.Println("MakeSumList elapsed: ", elapsed)
	// for _, sum := range sumList.list {
	// 	//println(sum.sum1)
	// 	fmt.Printf("%x   %s\n", sum.sum1, sum.sum2[0].sum2)
	// }
	// return true
	//step1  run in server
	// blockMap := make(map[string][]int)

	// for i := step; i <= len(f1)-step; i += step {

	//h := hash(f1[i : i+step])

	//blockMap[h] = append(blockMap[h], i)
	// }

	//step2 run in client
	//t = time.Now()
	patchList := MakePatch([]byte(f2), sumList)
	//elapsed = time.Since(t)
	//fmt.Println("MakePatch elapsed: ", elapsed)

	//step3 run in server
	//t = time.Now()
	rebuildData := RebuildFile(f1, patchList)

	// err := ioutil.WriteFile("./test/out.txt", rebuildData.Bytes(), 0644)
	// if err != nil {
	// 	panic(err)
	// }
	//elapsed = time.Since(t)
	//fmt.Println("RebuildFile elapsed: ", elapsed)
	//println("result: " + string(result.Bytes()))
	return bytes.Equal(rebuildData.Bytes(), f2)
	//return result.Bytes() == f2
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	//return string(b)
	return *(*string)(unsafe.Pointer(&b))
}

func main() {
	defer profile.Start().Stop()
	rand.Seed(time.Now().UnixNano())

	t1 := time.Now() // get current time
	for i := 0; i < 10000; i++ {
		f1 := RandString(rand.Intn(100000))
		f2 := RandString(rand.Intn(100000))

		//f1 := "bacbccbbcaacabbbcbbcabaaaabacbcaababbbcabbaccacaabaccacacbbccbbaacbcbbccabaaac"
		//f2 := "bbbbbbacbcacbbcbcbbbccabccbcbbcacbcccaabaaacbcbaabbcbbbcacabbbccsfafsefasccccc"

		//println(f1 + " <-----> " + f2)

		// fileData1, err := ioutil.ReadFile("./test/a.txt")
		// if err != nil {
		// 	panic(err)
		// }
		// fileData2, err := ioutil.ReadFile("./test/b.txt")
		// if err != nil {
		// 	panic(err)
		// }

		if !Diff([]byte(f1), []byte(f2)) {
			panic(f1 + "  " + f2)
		}

		if i%1000 == 0 {
			println(i)
		}
	}
	elapsed := time.Since(t1)
	fmt.Println("App elapsed: ", elapsed)

}

var h = md5.New()

func md5sum(input []byte) string {
	h.Reset()
	h.Write(input)
	return *(*string)(unsafe.Pointer(&input))
	//return string(h.Sum(nil))
}

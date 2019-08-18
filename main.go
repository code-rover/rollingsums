package main

import (
	"container/list"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
)

type Block struct {
	pos int
}

type Patch struct {
	data string
	pos  int
	len  int
}

var step = 5

func min(x int, y int) int {
	if x < y {
		return x
	}
	return y
}

func GenerateFile(origin string, patchList *list.List) string {
	result := ""
	for e := patchList.Front(); e != nil; e = e.Next() {
		patch := e.Value.(*Patch)
		if patch.pos == -1 {
			result += patch.data
			//println(patch.data)
		} else {
			result += origin[patch.pos : patch.pos+patch.len]
		}
	}
	return result
}

func MakePatch(f2 string, blockMap map[string][]int) *list.List {
	dataLen := len(f2)
	patchList := list.New()
	var backItem *Patch

	var bufA = -1 //差异开始段位置
	var bufB = -1 //差异结束段位置

	i := 0
	for i = 0; i+step <= dataLen; {
		backItem = nil
		if patchList.Back() != nil {
			backItem = patchList.Back().Value.(*Patch)
		}

		h := md5sum([]byte(f2[i : i+step]))
		if v, ok := blockMap[h]; ok {
			if bufA != -1 {
				backItem.data += f2[bufA:bufB]
				bufA = -1
				bufB = -1
			}
			//println(f2[i : i+step])
			//fmt.Printf("find: %s   %d   %s\n", h, v[0], f2[i:i+step])

			//优化 队列上一个元素不是字符串 或  间断块
			if backItem == nil || backItem.pos == -1 || v[0] != backItem.pos+backItem.len {
				backItem = &Patch{
					pos: v[0],
					len: step,
				}
				patchList.PushBack(backItem)
			} else {
				backItem.len += step
			}

			i += step

		} else {
			if backItem == nil || backItem.pos > -1 {
				backItem = &Patch{
					pos: -1,
				}
				patchList.PushBack(backItem)
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
		backItem.data += f2[bufA:bufB]
	}

	//剩余block处理
	if i+step > dataLen { //不足一个block的剩余
		if backItem == nil || backItem.pos > -1 {
			backItem = &Patch{pos: -1}
			patchList.PushBack(backItem)
		}
		backItem.data += f2[i:len(f2)]
	}

	return patchList
}

type SumInfo struct {
	sum1 uint32
	sum2 []string
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

	a := 1
	b := 0
	for i := 0; i < step; i++ {
		println(data[i])
		a += int(data[i])
		b += a
	}
	a %= 65521
	b %= 65521

	sum1 := uint32(b<<16 | a&0xffff)
	sum2 := md5sum([]byte(data[0:step]))

	sumMap[sum1] = &SumInfo{
		sum1: sum1,
		sum2: []string{sum2},
	}

	for i := step; i < len(data); i++ {
		a -= int(data[i-step])
		a += int(data[i])
		b -= step * int(data[i-step])
		b--
		b += a
		a %= 65521
		b %= 65521
		sum1 = uint32(b<<16 | a&0xffff)
		sum2 = md5sum([]byte(data[i-step : i]))

		if _, ok := sumMap[sum1]; !ok {
			sumMap[sum1] = &SumInfo{
				sum1: sum1,
			}
		}
		sumMap[sum1].sum2 = append(sumMap[sum1].sum2, sum2)
	}

	for _, v := range sumMap {
		sumList.list = append(sumList.list, v)
	}

	return &sumList
}

func Diff(f1 string, f2 string) bool {

	sumList := MakeSumList([]byte(f1))
	for _, sum := range sumList.list {
		//println(sum.sum1)
		fmt.Printf("%x   %s\n", sum.sum1, sum.sum2[0])
	}
	return true
	//step1  run in server
	blockMap := make(map[string][]int)

	for i := step; i <= len(f1)-step; i += step {

		//h := hash(f1[i : i+step])

		//blockMap[h] = append(blockMap[h], i)
	}

	//step2 run in client
	patchList := MakePatch(f2, blockMap)

	//step3 run in server
	result := GenerateFile(f1, patchList)
	//println("result: " + result)
	return result == f2
}

var letterRunes = []rune("abc")

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	t1 := time.Now() // get current time
	for i := 0; i < 1; i++ {
		f1 := RandString(rand.Intn(1000))
		f2 := RandString(rand.Intn(100))

		f1 = "abcdefghijklmnopqrst"
		f2 = "cbbbabbbbcbbaccababbbacbcccaaacabbbaabccbcaaaaabbbcbcbbaccaaabcabbbcbbbbcbbbaabc"

		//println(f1 + " <-----> " + f2)

		if !Diff(f1, f2) {
			panic(f1 + "  " + f2)
		}

		// if i%1000 == 0 {
		// 	println(i)
		// }
	}
	elapsed := time.Since(t1)
	fmt.Println("App elapsed: ", elapsed)

}

func md5sum(input []byte) string {
	h := md5.New()
	h.Write(input)
	//return hex.EncodeToString(h.Sum(nil))[8:24]
	return hex.EncodeToString(h.Sum(nil))
}

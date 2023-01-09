/*
 * TencentBlueKing is pleased to support the open source community by making 蓝鲸智云 - gopkg available.
 * Copyright (C) 2017 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 *	http://opensource.org/licenses/MIT
 *
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 * to the current version of the project delivered to anyone in the future.
 */

package mapx

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const (
	// ActionAdd 新增
	ActionAdd = "Add"
	// ActionChange 有修改
	ActionChange = "Change"
	// ActionRemove 移除
	ActionRemove = "Remove"
)

const (
	// NodeTypeIndex 节点类型（下标）
	NodeTypeIndex = "index"
	// NodeTypeKey 节点类型（键）
	NodeTypeKey = "key"
)

// Node Map 树节点
type Node struct {
	Type  string
	Index string
	Key   string
}

// NewIdxNode ...
func NewIdxNode(index int) Node {
	return Node{Type: NodeTypeIndex, Index: strconv.Itoa(index)}
}

// NewKeyNode ...
func NewKeyNode(key string) Node {
	return Node{Type: NodeTypeKey, Key: key}
}

// DiffRet Diff 结果
type DiffRet struct {
	Action string
	Dotted string
	OldVal interface{}
	NewVal interface{}
}

// NewDiffRet ...
func NewDiffRet(action string, nodes []Node, old, new interface{}) DiffRet {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case NodeTypeKey:
			// 若 key 中包含 `.`，则添加小括号以区分
			if strings.Contains(n.Key, ".") {
				b.WriteString(".(" + n.Key + ")")
			} else {
				b.WriteString("." + n.Key)
			}
		case NodeTypeIndex:
			b.WriteString("[" + n.Index + "]")
		}
	}
	dotted := strings.Trim(b.String(), ".")
	return DiffRet{Action: action, Dotted: dotted, OldVal: old, NewVal: new}
}

// Repr 转换结果为字符串
func (r *DiffRet) Repr() string {
	ret := fmt.Sprintf("%s %s: ", r.Action, r.Dotted)
	switch r.Action {
	case ActionAdd:
		ret += fmt.Sprintf("%v", r.NewVal)
	case ActionChange:
		ret += fmt.Sprintf("%v -> %v", r.OldVal, r.NewVal)
	case ActionRemove:
		ret += fmt.Sprintf("%v", r.OldVal)
	}
	return ret
}

// DiffRetList Diff 结果列表
type DiffRetList []DiffRet

// Len ...
func (l DiffRetList) Len() int {
	return len(l)
}

// Less 按类型排序，如果规则相同，则按照 Dotted 排序
func (l DiffRetList) Less(i, j int) bool {
	cmpRet := strings.Compare(l[i].Action, l[j].Action)
	if cmpRet == 0 {
		return strings.Compare(l[i].Dotted, l[j].Dotted) < 0
	}
	return cmpRet < 0
}

// Swap ...
func (l DiffRetList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// Differ Map 对比器，用于检查新旧 Map 中键值的差别
type Differ struct {
	old  map[string]interface{}
	new  map[string]interface{}
	rets DiffRetList
}

// NewDiffer ...
func NewDiffer(old, new map[string]interface{}) *Differ {
	return &Differ{old: old, new: new, rets: DiffRetList{}}
}

// Do 执行 Diff
func (d *Differ) Do() DiffRetList {
	d.handleMap(d.old, d.new, []Node{})
	sort.Sort(d.rets)
	return d.rets
}

func (d *Differ) handleMap(old, new map[string]interface{}, nodes []Node) {
	intersection, addition, deletion := []string{}, []string{}, []string{}
	for key := range old {
		if ExistsKey(new, key) {
			intersection = append(intersection, key)
		}
	}
	for key := range new {
		if !ExistsKey(old, key) {
			addition = append(addition, key)
		}
	}
	for key := range old {
		if !ExistsKey(new, key) {
			deletion = append(deletion, key)
		}
	}

	// intersection
	for _, key := range intersection {
		curNodes := append(nodes, NewKeyNode(key))
		d.handle(old[key], new[key], curNodes)
	}

	// addition
	for _, key := range addition {
		ret := NewDiffRet(ActionAdd, append(nodes, NewKeyNode(key)), nil, new[key])
		d.rets = append(d.rets, ret)
	}

	// deletion
	for _, key := range deletion {
		ret := NewDiffRet(ActionRemove, append(nodes, NewKeyNode(key)), old[key], nil)
		d.rets = append(d.rets, ret)
	}
}

func (d *Differ) handleList(old, new []interface{}, nodes []Node) {
	oldLen, newLen, minLen := len(old), len(new), len(old)
	if newLen < oldLen {
		minLen = newLen
	}

	// intersection
	for idx := 0; idx < minLen; idx++ {
		d.handle(old[idx], new[idx], append(nodes, NewIdxNode(idx)))
	}

	// addition
	for idx := minLen; idx < newLen; idx++ {
		ret := NewDiffRet(ActionAdd, append(nodes, NewIdxNode(idx)), nil, new[idx])
		d.rets = append(d.rets, ret)
	}

	// deletion
	for idx := minLen; idx < oldLen; idx++ {
		ret := NewDiffRet(ActionRemove, append(nodes, NewIdxNode(idx)), old[idx], nil)
		d.rets = append(d.rets, ret)
	}
}

func (d *Differ) handle(old, new interface{}, nodes []Node) {
	oldMap, oldIsMap := old.(map[string]interface{})
	newMap, newIsMap := new.(map[string]interface{})
	if oldIsMap && newIsMap {
		d.handleMap(oldMap, newMap, nodes)
		return
	}

	oldList, oldIsList := old.([]interface{})
	newList, newIsList := new.([]interface{})
	if oldIsList && newIsList {
		d.handleList(oldList, newList, nodes)
		return
	}

	if !reflect.DeepEqual(old, new) {
		ret := NewDiffRet(ActionChange, nodes, old, new)
		d.rets = append(d.rets, ret)
	}
}

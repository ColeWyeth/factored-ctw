// Package ctw provides an implementation of the Context Tree Weighting algorithm.
// Also contained is an implementation of the Rissanen-Langdon Arithmetic Coding algorithm, which is combined with Context Tree Weighting to create a lossless compression/decompression utility.
//
// Below is an example of using this package to compress Lincoln's Gettysburg address:
//
//	go run compress/main.go gettysburg.txt > gettys.ctw
//	cat gettys.ctw | go run decompress/main.go > gettys.dctw
//	diff gettysburg.txt gettys.dctw
//
// Reference:
// F.M.J. Willems and Tj. J. Tjalkens, Complexity Reduction of the Context-Tree Weighting Algorithm: A Study for KPN Research, Technical University of Eindhoven, EIDMA Report RS.97.01.
package ctw

import (
	"log"
	"math"
)

// logaddexp performs log(exp(x) + exp(y))
func logaddexp(x, y float64) float64 {
	tmp := x - y
	if tmp > 0 {
		return x + math.Log1p(math.Exp(-tmp))
	} else if tmp <= 0 {
		return y + math.Log1p(math.Exp(tmp))
	} else {
		// Nans, or infinities of the same sign involved
		log.Printf("logaddexp %f %f", x, y)
		return x + y
	}
}

// treeNode represents a suffix in a Context Tree Weighting.
// It holds the log probability of the source sequence given the suffix represented by the node.
type treeNode struct {
	LogProb float64 // log probability of suffix

	A    uint32  // number of zerRos with suffix
	B    uint32  // number of ones with suffix
	Lktp float64 // log probability of the Krichevsky-Trofimov (KT) Estimation, given our current number of zeros and ones.

	Left  *treeNode // the sub-suffix that ends with one
	Right *treeNode // the sub-suffix that ends with zero
}

type snapshot struct {
	node  *treeNode
	state treeNode
	isNew bool
}

func revert(traversed []snapshot) {
	for i, ss := range traversed {
		node := ss.node
		node.Lktp = ss.state.Lktp
		node.A = ss.state.A
		node.B = ss.state.B
		node.LogProb = ss.state.LogProb

		// The memory releasing logic below saves memory.
		// However, it might increase GC times if the released memory is added back again.
		// This happens when our predictions are faily consistent with the eventually arriving data.
		// Here we emphasize performance by not doing this memory saving optimization.
		//
		if i < len(traversed)-1 {
			next := traversed[i+1]
			if next.isNew {
				if next.node == node.Right {
					node.Right = nil
				} else {
					node.Left = nil
				}
				break
			}
		}
	}
}

// update updates the tree according to the rules of CTW.
// Root is the root of the context tree.
// Bits is the last few bits of the sequence, len(bits) should be the depth of the tree.
// Bit is the new bit following the sequence.
func update(root *treeNode, bits []int, bit int) []snapshot {
	if bit != 0 && bit != 1 {
		log.Fatalf("wrong bit %d", bit)
	}

	// Update the counts of zeros and ones of each node.
	traversed := []snapshot{}
	node := root
	traversed = append(traversed, snapshot{node: node, state: *node, isNew: false})
	krichevskyTrofimov(node, bit)

	for d := 0; d < len(bits); d++ {
		isNew := false
		if bits[len(bits)-1-d] == 0 {
			if node.Right == nil {
				node.Right = &treeNode{}
				isNew = true
			}
			node = node.Right
		} else {
			if node.Left == nil {
				node.Left = &treeNode{}
				isNew = true
			}
			node = node.Left
		}

		traversed = append(traversed, snapshot{node: node, state: *node, isNew: isNew})
		krichevskyTrofimov(node, bit)
	}

	// Update the actual node probabilities.
	for i := len(traversed) - 1; i >= 0; i-- {
		ss := traversed[i]
		node := ss.node

		if node.Left != nil || node.Right != nil {
			var lp float64 = 0
			if node.Left != nil {
				lp = node.Left.LogProb
			}
			var rp float64 = 0
			if node.Right != nil {
				rp = node.Right.LogProb
			}
			w := 0.5
			node.LogProb = logaddexp(math.Log(w)+node.Lktp, math.Log(1-w)+lp+rp)
		} else {
			node.LogProb = node.Lktp
		}
	}

	return traversed
}

// krichevskyTrofimov updates the Krichevsky-Trofimov estimate of a node given a new observed bit.
func krichevskyTrofimov(node *treeNode, bit int) {
	a := float64(node.A)
	b := float64(node.B)
	if bit == 0 {
		node.Lktp = node.Lktp + math.Log(a+0.5) - math.Log(a+b+1)
		node.A += 1
	} else {
		node.Lktp = node.Lktp + math.Log(b+0.5) - math.Log(a+b+1)
		node.B += 1
	}
}

// A CTW is a Context Tree Weighting based probabilistic model for binary data.
// CTW implements the arithmetic coding Model interface.
type CTW struct {
	Bits []int
	Root *treeNode
}

// NewCTW returns a new CTW whose context tree's depth is len(bits).
// The prior context of the tree is given by bits.
func NewCTW(bits []int) *CTW {
	model := &CTW{
		Bits: bits,
		Root: &treeNode{},
	}
	return model
}

// Prob0 returns the probability that the next bit be zero.
func (model *CTW) Prob0() float64 {
	before := model.Root.LogProb
	traversal := update(model.Root, model.Bits, 0)
	after := model.Root.LogProb

	revert(traversal)

	return math.Exp(after - before)
}

// Observe updates the context tree, given that the sequence is followed by bit.
func (model *CTW) Observe(bit int) {
	model.observe(bit)
}

func (model *CTW) observe(bit int) []snapshot {
	traversal := update(model.Root, model.Bits, bit)
	for i := 1; i < len(model.Bits); i++ {
		model.Bits[i-1] = model.Bits[i]
	}
	model.Bits[len(model.Bits)-1] = bit
	return traversal
}

// A CTWReverter is a CTW model that allows reverting to its previous state.
// This is useful for predicting several steps ahead, while keeping the model's original state intact.
type CTWReverter struct {
	model      *CTW
	bits       []int
	traversals [][]snapshot
}

func NewCTWReverter(model *CTW) *CTWReverter {
	cr := &CTWReverter{}
	cr.model = model
	return cr
}

func (cr *CTWReverter) Prob0() float64 {
	return cr.model.Prob0()
}

func (cr *CTWReverter) Observe(bit int) {
	cr.bits = append(cr.bits, cr.model.Bits[0])
	cr.traversals = append(cr.traversals, cr.model.observe(bit))
}

func (cr *CTWReverter) Unobserve() {
	// Revert the tree.
	tvIdx := len(cr.traversals) - 1
	revert(cr.traversals[tvIdx])
	cr.traversals = cr.traversals[:tvIdx]

	// Revert the context bits.
	for i := len(cr.model.Bits) - 1; i > 0; i-- {
		cr.model.Bits[i] = cr.model.Bits[i-1]
	}
	btIdx := len(cr.bits) - 1
	cr.model.Bits[0] = cr.bits[btIdx]
	cr.bits = cr.bits[:btIdx]
}

// An FCTW is a probabilistic model for structured binary data constructed from
// CTW models for each bit position within a block.
// FCTW implements the arithmetic coding Model interface
type FCTW struct {
	Trees     []*CTW
	Block_len int
	Index     int
}

// NewFCTW returns a new FCTW whose context tree's depth is len(bits).
// The prior context of the trees is given by bits.
// The initial index position is len(bits) mod block_len.
func NewFCTW(block_len int, bits []int) *FCTW {
	trees := make([]*CTW, block_len)
	for i := 0; i < block_len; i++ {
		trees[i] = NewCTW(bits)
	}
	index := len(bits) % block_len
	model := &FCTW{
		trees,
		block_len,
		index,
	}
	return model
}

// Prob0 returns the probability that the next bit be zero.
func (model *FCTW) Prob0() float64 {
	tree := model.Trees[model.Index]
	before := tree.Root.LogProb
	traversal := update(tree.Root, tree.Bits, 0)
	after := tree.Root.LogProb

	revert(traversal)

	return math.Exp(after - before)
}

// Observe updates the context tree, given that the sequence is followed by bit.
func (model *FCTW) Observe(bit int) {
	for i := 0; i < model.Block_len; i++ {
		model.Trees[i].observe(bit)
	}
	model.Index = (model.Index + 1) % model.Block_len
}

// A VOM (variable order Markov) model is an element of the mixture considered by CTW.
// VOM implements the arithmetic coding Model interface
// A VOM is produced by context-tree maximization.
type VOM struct {
	Root *VOMNode
	Bits []int
}

func NewVOM(bits []int) *VOM {
	root := &VOMNode{
		true,
		0.5,
		0.0,
		nil,
		nil,
	}
	return &VOM{
		root,
		bits,
	}
}

type VOMNode struct {
	Leaf       bool
	CondProb0  float64
	MaxLogProb float64
	Child0     *VOMNode
	Child1     *VOMNode
}

// For the moment we only care about creating a VOM model from a CTW model
func ToVOM(model *CTW) *VOM {
	// set Bits from CTW
	// then set root by recursively converting CTW Root.
	bits := make([]int, len(model.Bits))
	_ = copy(bits, model.Bits)
	vom_model := &VOM{
		ToVOMNode(model.Root),
		bits,
	}
	return vom_model
}

func ToVOMNode(node *treeNode) *VOMNode {
	a := float64(node.A)
	b := float64(node.B)
	ktp := (a + 0.5) / (a + b + 1.0)
	if node.Left == nil && node.Right == nil {
		// fmt.Print("Reached terminal node")
		return &VOMNode{
			true,
			ktp,
			node.Lktp,
			nil,
			nil,
		}
	}
	var LeftVOM *VOMNode = nil
	var mlp float64 = 0.0
	if node.Left != nil {
		LeftVOM = ToVOMNode(node.Left)
		mlp = LeftVOM.MaxLogProb
	}
	var RightVOM *VOMNode = nil
	var mrp float64 = 0.0
	if node.Right != nil {
		RightVOM = ToVOMNode(node.Right)
		mrp = RightVOM.MaxLogProb
	}
	if node.Lktp >= mlp+mrp {
		// fmt.Print("The lktp was higher!")
		return &VOMNode{
			true,
			ktp,
			node.Lktp,
			nil,
			nil,
		}
	} else {
		// fmt.Print("The lktp was lower!")
		if LeftVOM == nil {
			LeftVOM = &VOMNode{
				true,
				0.5,
				0.0,
				nil,
				nil,
			}
		}
		if RightVOM == nil {
			RightVOM = &VOMNode{
				true,
				0.5,
				0.0,
				nil,
				nil,
			}
		}
		return &VOMNode{
			false,
			-1.0, // This value is irrelevant
			mlp + mrp,
			RightVOM,
			LeftVOM,
		}
	}

}

// if node.Left != nil || node.Right != nil {
// 	var lp float64 = 0
// 	if node.Left != nil {
// 		lp = node.Left.LogProb
// 	}
// 	var rp float64 = 0
// 	if node.Right != nil {
// 		rp = node.Right.LogProb
// 	}
// 	w := 0.5
// 	node.LogProb = logaddexp(math.Log(w)+node.Lktp, math.Log(1-w)+lp+rp)
// } else {
// 	node.LogProb = node.Lktp
// }

// treeNode represents a suffix in a Context Tree Weighting.
// It holds the log probability of the source sequence given the suffix represented by the node.
// type treeNode struct {
// 	LogProb float64 // log probability of suffix

// 	A    uint32  // number of zerRos with suffix
// 	B    uint32  // number of ones with suffix
// 	Lktp float64 // log probability of the Krichevsky-Trofimov (KT) Estimation, given our current number of zeros and ones.

// 	Left  *treeNode // the sub-suffix that ends with one
// 	Right *treeNode // the sub-suffix that ends with zero
// }

// Prob0 returns the probability that the next bit be zero.
func (model *VOM) Prob0() float64 {
	// fmt.Println("Calculating Prob0 with VOM")
	currNode := model.Root
	for d := 0; d < len(model.Bits); d++ {
		if currNode.Leaf {
			return currNode.CondProb0
		}
		if model.Bits[len(model.Bits)-d-1] == 0 {
			currNode = currNode.Child0
		} else {
			currNode = currNode.Child1
		}
	}
	// if currNode.Leaf {
	// 	fmt.Println("Ended at a terminal leaf")
	// }
	return currNode.CondProb0
}

// Observe updates the model, given that the sequence is followed by bit.
func (model *VOM) Observe(bit int) {
	for i := 1; i < len(model.Bits); i++ {
		model.Bits[i-1] = model.Bits[i]
	}
	model.Bits[len(model.Bits)-1] = bit
}

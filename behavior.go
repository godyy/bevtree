package bevtree

import "github.com/godyy/bevtree/internal/assert"

type Bev interface {
	OnInit(*Env)
	OnUpdate(*Env) Result
	OnTerminate(*Env)
}

type BevDefiner interface {
	CreateBev() Bev
	DestroyBev(Bev)
}

type BevNode struct {
	nodeBase
	bevDef BevDefiner
}

func NewBev(bevDef BevDefiner) *BevNode {
	assert.NilArg(bevDef, "bevDef")

	return &BevNode{
		bevDef: bevDef,
	}
}

func (BevNode) ChildCount() int           { return 0 }
func (BevNode) AddChild(node)             {}
func (BevNode) RemoveChild(node)          {}
func (BevNode) AddChildBefore(_, _ node)  {}
func (BevNode) AddChildAfter(_, _ node)   {}
func (BevNode) MoveChildBefore(_, _ node) {}
func (BevNode) MoveChildAfter(_, _ node)  {}
func (BevNode) FirstChild() node          { return nil }
func (BevNode) LastChild() node           { return nil }

func (b *BevNode) createTask(parent task) task {
	bev := b.bevDef.CreateBev()
	assert.Nil(bev, "bevDef create nil behavior")

	return bevTaskPool.get().(*bevTask).ctr(b, parent, bev)
}

func (b *BevNode) destroyTask(t task) {
	bt := t.(*bevTask)
	b.bevDef.DestroyBev(bt.bev)
	bt.dtr()
	bevTaskPool.put(t)
}

var bevTaskPool = newTaskPool(func() task { return newBevTask() })

type bevTask struct {
	taskBase
	bev Bev
}

func newBevTask() *bevTask {
	return new(bevTask)
}

func (t *bevTask) ctr(node *BevNode, parent task, bev Bev) task {
	assert.NilArg(bev, "bev")

	t.taskBase.ctr(node, parent)
	t.bev = bev

	return t
}

func (t *bevTask) dtr() {
	t.bev = nil
	t.taskBase.dtr()
}

func (t *bevTask) isBehavior() bool { return true }

func (t *bevTask) update(e *Env) Result {
	st := t.getStatus()

	if debug {
		assert.NotEqual(st, sDestroyed, "bevTask already destroyed")
	}

	// update seri.
	t.latestUpdateSeri = e.getUpdateSeri()

	lzStop := t.getLZStop()

	// lazy stop before update.
	if lzStop == lzsBeforeUpdate {
		return t.doLazyStop(e)
	}

	// init.
	if st != sRunning {
		t.bev.OnInit(e)
	}

	// update.
	result := t.bev.OnUpdate(e)

	// lazy stop after update.
	if lzStop == lzsAfterUpdate {
		return t.doLazyStop(e)
	}

	if result == RRunning {
		t.setStatus(sRunning)
	} else {
		// terminate.
		t.bev.OnTerminate(e)
		t.setStatus(sNone)
	}

	return result
}

func (t *bevTask) stop(e *Env) {
	if !t.isRunning() {
		return
	}

	t.bev.OnTerminate(e)
	t.setStatus(sStopped)
	t.setLZStop(lzsNone)
}

func (t *bevTask) doLazyStop(e *Env) Result {
	t.stop(e)
	return RFailure
}

func (t *bevTask) childOver(_ task, _ Result, _ *Env) Result {
	panic("should not be called")
}

func (t *bevTask) destroy() {
	if debug {
		assert.NotEqual(t.getStatus(), sDestroyed, "bevTask already destroyed")
		assert.False(t.isRunning(), "bevTask still running")
	}

	t.st = sDestroyed
	t.node.destroyTask(t)
}

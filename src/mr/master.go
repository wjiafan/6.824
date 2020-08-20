package mr

import "log"
import "net"
import "os"
import "net/rpc"
import "net/http"
import "sync"
import "strconv"
import "time"


type Master struct {
	// Your definitions here.
	AllFilesName        map[string]int
	MapTaskNumCount     int
	NReduce             int               // n reduce task
	//
	InterFIlename       [][]string        // store location of intermediate files
	MapFinished         bool
	ReduceTaskStatus    map[int]int      // about reduce tasks' status
	ReduceFinished      bool              // Finish the reduce task
	RWLock              *sync.RWMutex
}

// tasks' status
// UnAllocated    ---->     UnAllocated to any worker
// Allocated      ---->     be allocated to a worker
// Finished       ---->     worker finish the map task
const (
	UnAllocated = iota
	Allocated
	Finished
)

// global
var maptasks chan string          // chan for map task
var reducetasks chan int          // chan for reduce task

// generateTask : create tasks
func (m *Master) generateTask() {
	for k,v := range m.AllFilesName {
		if v == UnAllocated {
			maptasks <- k          // add task to channel
		}
	}
	ok := false
	for !ok {
		ok = checkAllMapTask(m)    // check if all map tasks have finished
	}

	m.MapFinished = true

	for k,v := range m.ReduceTaskStatus {
		if v == UnAllocated {
			reducetasks <- k
		}
	}

	ok = false
	for !ok {
		ok = checkAllReduceTask(m)
	}
	m.ReduceFinished = true
}

// checkAllMapTask : check if all map tasks are finished
func checkAllMapTask(m *Master) bool {
	m.RWLock.RLock()
	defer m.RWLock.RUnlock()
	for _,v := range m.AllFilesName {
		if v != Finished {
			return false
		}
	}
	return true
}

func checkAllReduceTask(m *Master) bool {
	m.RWLock.RLock()
	defer m.RWLock.RUnlock()
	for _, v := range m.ReduceTaskStatus {
		if v != Finished {
			return false
		}
	}
	return true
}


// Your code here -- RPC handlers for the worker to call.
func (m *Master) MyCallHandler(args *MyArgs, reply *MyReply) error {
	msgType := args.MessageType // worker 发送的消息的类型
	switch(msgType) {
	case MsgForTask:
		select {
		case filename := <- maptasks:
			// allocate map task
			reply.Filename = filename
			reply.MapNumAllocated = m.MapTaskNumCount
			reply.NReduce = m.NReduce
			reply.TaskType = "map"

			m.RWLock.Lock()
			m.AllFilesName[filename] = Allocated
			m.MapTaskNumCount++
			m.RWLock.Unlock()
			go m.timerForWorker("map",filename)
			return nil

		case reduceNum := <- reducetasks:
			// allocate reduce task
			reply.TaskType = "reduce"
			reply.ReduceFileList = m.InterFIlename[reduceNum]
			reply.NReduce = m.NReduce
			reply.ReduceNumAllocated = reduceNum

			m.RWLock.Lock()
			m.ReduceTaskStatus[reduceNum] = Allocated
			m.RWLock.Unlock()
			go m.timerForWorker("reduce", strconv.Itoa(reduceNum))
			return nil
		}
	case MsgForFinishMap:
		// finish a map task
		m.RWLock.Lock()
		defer m.RWLock.Unlock()
		m.AllFilesName[args.MessageCnt] = Finished   // set status as finish
	case MsgForFinishReduce:
		// finish a reduce task
		index, _ := strconv.Atoi(args.MessageCnt)
		m.RWLock.Lock()
		defer m.RWLock.Unlock()
		m.ReduceTaskStatus[index] = Finished        // set status as finish
	}
	return nil
}

// MyInnerFileHandler : intermediate files' handler
func (m *Master) MyInnerFileHandler(args *MyIntermediateFile, reply *MyReply) error {
	nReduceNUm := args.NReduceType;
	filename := args.MessageCnt;

	// store themm
	m.InterFIlename[nReduceNUm] = append(m.InterFIlename[nReduceNUm], filename)
	return nil
}


//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (m *Master) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}


//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	// init channels
	maptasks = make(chan string, 5)
	reducetasks = make(chan int, 5)

	rpc.Register(m)
	rpc.HandleHTTP()

	// parallel run generateTask()
	go m.generateTask()

	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	ret := false
	// Your code here.
	ret = m.ReduceFinished

	return ret
}

//
// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{}

	// Your code here.
	m.InterFIlename = make([][]string, m.NReduce)
	m.AllFilesName = make(map[string]int)
	m.MapTaskNumCount = 0
	m.NReduce = nReduce
	m.MapFinished = false
	m.ReduceFinished = false
	m.ReduceTaskStatus = make(map[int]int)
	for _,v := range files {
		m.AllFilesName[v] = UnAllocated
	}

	for i := 0; i<nReduce; i++ {
		m.ReduceTaskStatus[i] = UnAllocated
	}

	m.server()
	return &m
}

func (m *Master)timerForWorker(taskType, identify string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if taskType == "map" {
				m.RWLock.Lock()
				m.AllFilesName[identify] = UnAllocated
				m.RWLock.Unlock()
				// 重新让任务加入channel
				maptasks <- identify
			} else if taskType == "reduce" {
				index, _ := strconv.Atoi(identify)
				m.RWLock.Lock()
				m.ReduceTaskStatus[index] = UnAllocated
				m.RWLock.Unlock()
				// 重新将任务加入channel
				reducetasks <- index
			}
			return
		default:
			if taskType == "map" {
				m.RWLock.RLock()
				if m.AllFilesName[identify] == Finished {
					m.RWLock.RUnlock()
					return
				} else {
					m.RWLock.RUnlock()
				}
			} else if taskType == "reduce" {
				index, _ := strconv.Atoi(identify)
				m.RWLock.RLock()
				if m.ReduceTaskStatus[index] == Finished {
					m.RWLock.RUnlock()
					return
				} else {
					m.RWLock.RUnlock()
				}
			}
		}
	}
}

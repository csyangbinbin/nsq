package consistence

import (
	"errors"
	"fmt"
	"github.com/absolute8511/nsq/internal/test"
	"github.com/absolute8511/nsq/nsqd"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
)

type fakeNsqdLeadership struct {
	clusterID      string
	regData        map[string]*NsqdNodeInfo
	fakeTopicsData map[string]map[int]*TopicCoordinator
}

func NewFakeNSQDLeadership() NSQDLeadership {
	return &fakeNsqdLeadership{
		regData:        make(map[string]*NsqdNodeInfo),
		fakeTopicsData: make(map[string]map[int]*TopicCoordinator),
	}
}

func (self *fakeNsqdLeadership) InitClusterID(id string) {
	self.clusterID = id
}

func (self *fakeNsqdLeadership) Register(nodeData NsqdNodeInfo) error {
	self.regData[nodeData.GetID()] = &nodeData
	return nil
}

func (self *fakeNsqdLeadership) Unregister(nodeData NsqdNodeInfo) error {
	delete(self.regData, nodeData.GetID())
	return nil
}

func (self *fakeNsqdLeadership) AcquireTopicLeader(topic string, partition int, nodeData NsqdNodeInfo) error {
	t, ok := self.fakeTopicsData[topic]
	var tc *TopicCoordinator
	if ok {
		if tc, ok = t[partition]; ok {
			if tc.topicLeaderSession.LeaderNode != nil {
				return errors.New("topic leader already exist.")
			}
			tc.topicLeaderSession.LeaderNode = &nodeData
			tc.topicLeaderSession.LeaderEpoch++
			tc.topicLeaderSession.Session = nodeData.GetID() + strconv.Itoa(int(tc.topicLeaderSession.LeaderEpoch))
			tc.topicInfo.ISR = append(tc.topicInfo.ISR, nodeData.GetID())
			tc.topicInfo.Leader = nodeData.GetID()
			tc.topicInfo.Epoch++
		} else {
			tc = &TopicCoordinator{}
			tc.topicInfo.Name = topic
			tc.topicInfo.Partition = partition
			tc.localDataLoaded = true
			tc.topicInfo.Leader = nodeData.GetID()
			tc.topicInfo.ISR = append(tc.topicInfo.ISR, nodeData.GetID())
			tc.topicInfo.Epoch++
			tc.topicLeaderSession.LeaderNode = &nodeData
			tc.topicLeaderSession.LeaderEpoch++
			tc.topicLeaderSession.Session = nodeData.GetID() + strconv.Itoa(int(tc.topicLeaderSession.LeaderEpoch))
			t[partition] = tc
		}
	} else {
		tmp := make(map[int]*TopicCoordinator)
		tc = &TopicCoordinator{}
		tc.topicInfo.Name = topic
		tc.topicInfo.Partition = partition
		tc.localDataLoaded = true
		tc.topicInfo.Leader = nodeData.GetID()
		tc.topicInfo.ISR = append(tc.topicInfo.ISR, nodeData.GetID())
		tc.topicInfo.Epoch++
		tc.topicLeaderSession.LeaderNode = &nodeData
		tc.topicLeaderSession.LeaderEpoch++
		tc.topicLeaderSession.Session = nodeData.GetID() + strconv.Itoa(int(tc.topicLeaderSession.LeaderEpoch))
		tmp[partition] = tc
		self.fakeTopicsData[topic] = tmp
	}
	return nil
}

func (self *fakeNsqdLeadership) ReleaseTopicLeader(topic string, partition int) error {
	t, ok := self.fakeTopicsData[topic]
	if ok {
		delete(t, partition)
	}
	return nil
}

func (self *fakeNsqdLeadership) WatchLookupdLeader(key string, leader chan *NsqLookupdNodeInfo, stop chan struct{}) error {
	return nil
}

func (self *fakeNsqdLeadership) GetTopicInfo(topic string, partition int) (*TopicPartionMetaInfo, error) {
	t, ok := self.fakeTopicsData[topic]
	if ok {
		tc, ok2 := t[partition]
		if ok2 {
			return &tc.topicInfo, nil
		}
	}
	return nil, errors.New("topic not exist")
}

func startNsqdCoord(rpcport string, dataPath string, extraID string, nsqd *nsqd.NSQD) *NsqdCoordinator {
	nsqdCoord := NewNsqdCoordinator("127.0.0.1", "0", rpcport, extraID, dataPath, nsqd)
	nsqdCoord.leadership = NewFakeNSQDLeadership()
	err := nsqdCoord.Start()
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second)
	return nsqdCoord
}

func TestNsqdRPCClient(t *testing.T) {
	coordLog.level = 2
	tmpDir, err := ioutil.TempDir("", fmt.Sprintf("nsq-test-%d", time.Now().UnixNano()))
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	nsqdCoord := startNsqdCoord("0", tmpDir, "", nil)
	time.Sleep(time.Second * 2)
	client, err := NewNsqdRpcClient(nsqdCoord.rpcServer.rpcListener.Addr().String(), time.Second)
	test.Nil(t, err)
	var rspInt int32
	err = client.CallWithRetry("NsqdCoordinator.TestRpcCallNotExist", "req", &rspInt)
	test.NotNil(t, err)

	rsp, rpcErr := client.CallRpcTest("reqdata")
	test.NotNil(t, rpcErr)
	test.Equal(t, rsp, "reqdata")
	test.Equal(t, rpcErr.ErrCode, RpcNoErr)
	test.Equal(t, rpcErr.ErrMsg, "reqdata")
	test.Equal(t, rpcErr.ErrType, CoordCommonErr)
}
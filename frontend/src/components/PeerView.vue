<!--suppress EqualityComparisonWithCoercionJS -->
<template>
  <div style="background: transparent;padding: 10px 10px 0 5px;">
    <div style="background: transparent;line-height: initial;">
      <span style="color: white;font-size: 14px;margin-left: 30px">刷新 </span>
      <a-button shape="circle" size="small" @click="loadPeers" style="font-size: 13px;min-width: auto;width: 19px;height: 19px; margin: 0 0 2px 3px;background: transparent;border: lightseagreen  solid 1px;color: lightseagreen" >
        <template #icon><SyncOutlined /></template>
      </a-button>
      <a-input v-model:value="filterVal" class="search-box" placeholder="请输入终端名称或IP">
        <template #suffix>
          <SearchOutlined />
        </template>
      </a-input>
    </div>
    <a-table :columns="columns"
             :data-source="filterResult"
             class="peer-table"
             :pagination="false"
             :rowKey="record => record.peer_mac"
             :row-selection="{ selectedRowKeys: selectedKeys,onChange: onSelectChange, type: 'radio',columnWidth: 20}"
             @expand="rowExpand"
             :expandedRowKeys="expandedRowKeys"
             :customRow="rowClick">
      <template #bodyCell="{ column, text, record }">
        <template v-if="['peer_name', 'inter_addr'].includes(column.key)">
          <div>
            <a-input
                v-if="cacheData[record.key]"
                v-model:value="cacheData[record.key][column.dataIndex]"
                style="color: white;background: #003366"
            />
            <template v-else>
              {{ text }}
            </template>
          </div>
        </template>
        <template v-else-if="column.key === 'link_mode'">
          <span :style="{'color':record.online?'lime':'#7F8487'}" style="font-size: 13px">
            <span>{{ record.link_mode ? linkModes[record.link_mode]+' / ' : '未知 / '}}</span>
            <span>{{ record.link_quality>100 ? '?' : record.link_quality}}</span>
          </span>
        </template>
        <template v-else-if="column.key === 'dev_type'">
          <span>
            <img :src="record.state_img" style="width: 24px;height: 24px">
            <span v-if="record.online" style="color: lime;font-size: 13px">{{ record.connect_type ? ' P2P' : ' 转发'}}</span>
            <span v-else style="color: orangered;font-size: 13px"> &nbsp;离线</span>
          </span>
        </template>
        <template v-else-if="column.key === 'operation'">
          <div >
            <span v-if="cacheData[record.key]">
              <a-popconfirm title="是否保存?" okText="是" cancelText="否" @confirm.stop="savePeer(record.key)">
                <a style="color: white" @click.stop="()=>{}"><CheckOutlined/></a>
              </a-popconfirm>
              <a-typography-link @click.stop="cancelEditPeer(record.key)" style="margin-left: 5px;color: white"><CloseOutlined/></a-typography-link>
            </span>
            <span v-else>
              <a v-if="record.online && record.dev_type != 'windows'" @click.stop="editPeer(record.key)" style="color: white;cursor: hand" >
                <EditOutlined />
              </a>
            </span>
          </div>
        </template>
      </template>
      <template #expandedRowRender="{ record }">
        <div style="margin:5px auto">
          <a-tabs>
            <a-tab-pane key="1" tab="局域网节点">
              <a-table
                  :columns="linkColumns"
                  :data-source="record.links"
                  :pagination="false"
                  :locale="{emptyText:' '}"
                  style="height: 180px;font-size:13px;overflow: auto">
              </a-table>
            </a-tab-pane>
            <a-tab-pane key="2" tab="远程串口">
              <div>
                <div>
                  <a-button type="primary" size="small" style="background: lightseagreen;" @click="loadConnectedPorts(record)">
                    <template #icon><sync-outlined /></template>
                    刷新信息
                  </a-button>
                  <a-button type="primary" size="small" style="margin-left: 20px" @click="showConfigDialog('','')">
                    <template #icon><plus-circle-outlined /></template>
                    连接新串口
                  </a-button>
                </div>
                <a-table
                    :columns="portColumns"
                    :data-source="record.connected_ports"
                    :pagination="false"
                    :locale="{emptyText:' '}"
                    :row-key="record => record.name"
                    style="height: 180px;font-size:13px;overflow: auto">
                  <template #bodyCell="{ column,text, record }">
                    <template v-if="column.key === 'baud'">
                      {{ bauds.get(text) }}
                    </template>
                    <template v-else-if="column.key === 'parity'">
                      {{ parities.get(text) }}
                    </template>
                    <template v-else-if="column.key === 'size'">
                      {{ dataBits.get(text) }}
                    </template>
                    <template v-else-if="column.key === 'stop_bits'">
                      {{ stopBits.get(text) }}
                    </template>
                    <template v-else-if="column.key === 'operation'">
                      <div v-if="record" >
                        <a-tooltip type="light" title="设置参数" color="cyan">
                          <setting-filled style="margin-left:20px;color: white" @click="showConfigDialog(record.remote_name,record.name)"></setting-filled>
                        </a-tooltip>
                        <a-tooltip v-if="!record.isOpen"  type="light" title="打开串口" color="cyan">
                          <link-outlined :disabled="waiting"  v-if="!record.isOpen" style="margin-left:20px;color: lime" @click="openSerialPort(record.remote_name,record.name)"/>
                        </a-tooltip>
                        <a-tooltip v-if="record.isOpen" type="light" title="关闭串口" color="cyan">
                          <disconnect-outlined :disabled="waiting" v-if="record.isOpen" style="margin-left:20px;color: orangered" @click="closeSerialPort(record.remote_name)" />
                        </a-tooltip>
                      </div>
                    </template>
                  </template>
                </a-table>
              </div>
            </a-tab-pane>
          </a-tabs>
        </div>
      </template>
    </a-table>
    <a-modal v-model:visible="showConfig"  :maskClosable="false" :destroyOnClose="true" title="串口管理" style="width: 540px">
      <SerialConfig :peer="expandedRecord" :remote-port="remotePort" :local-port="localPort" @closeDialog="serialDialogClosed"></SerialConfig>
      <template #footer>
      </template>
    </a-modal>
  </div>
</template>
<script>

import SerialConfig from "@/components/SerialConfig";
import { cloneDeep } from 'lodash-es';
import { defineComponent, onMounted, reactive, ref, watch} from 'vue';
import {
  SearchOutlined, EditOutlined,SettingFilled,SyncOutlined, CheckOutlined, CloseOutlined,LinkOutlined,PlusCircleOutlined,DisconnectOutlined,
} from '@ant-design/icons-vue'
import {message} from "ant-design-vue";
import serialConfig from "@/components/SerialConfig.vue";

const columns = [
  {title: '名称', dataIndex: 'peer_name', width: '25%', key: 'peer_name'},
  {title: '终端地址', dataIndex: 'net_addr', width: '15%', key: 'net_addr'},
  {title: '内网地址', dataIndex: 'inter_addr', width: '15%', key: 'inter_addr'},
  {title: '联网信息', dataIndex: 'link_mode', width: '16%', key: 'link_mode'},
  {title: '终端状态', dataIndex: 'dev_type', width: '16%', key: 'dev_type'},
  {title: '操作', dataIndex: 'operation', key: 'operation', width: '12%', scopedSlots: {customRender: 'operation'}},
];
const linkColumns = [
  {title: 'IP地址', dataIndex: 'ip', key: 'ip', width: '33%', align: 'center'},
  {title: '发送', dataIndex: 'send', key: 'send', width: '33%', align: 'center'},
  {title: '接收', dataIndex: 'receive', key: 'receive', width: '33%', align: 'center'},
];

const portColumns = [
  {title: '远端串口', dataIndex: 'remote_name', key: 'remote_name',width: '10%',  align: 'center'},
  {title: '用户串口', dataIndex: 'user_port_name', key: 'name',width: '10%',  align: 'center'},
  {title: '波特率', dataIndex: 'baud', key: 'baud', width: '12%', align: 'center'},
  {title: '奇偶校验', dataIndex: 'parity', key: 'parity', width: '10%', align: 'center'},
  {title: '数字位', dataIndex: 'size', key: 'size', width: '10%', align: 'center'},
  {title: '停止位', dataIndex: 'stop_bits', key: 'stop_bits', width: '10%', align: 'center'},
  {title: '操作', dataIndex: 'operation', key: 'operation', width: '14%', align: 'center',scopedSlots: {customRender: 'operation'}},
];

export default defineComponent({
  computed: {
    serialConfig() {
      return serialConfig
    }
  },
  components: {
    SerialConfig,
    SearchOutlined,
    EditOutlined,
    SettingFilled,
    SyncOutlined,
    CheckOutlined,
    CloseOutlined,
    LinkOutlined,
    PlusCircleOutlined,
    DisconnectOutlined,
  },
  props: {
    peerState: {
      type: Number,
      default: 0
    },
  },
  setup(props) {
    let peers = ref([]);
    let linkModes = ["?","有线","WiFi","GPRS"]
    let filterResult = ref([]);
    let cacheData = reactive({});
    let selectedKeys = ref([]);
    let filterVal = ref('');
    let editingKey = '';
    // 串口处理信息
    let localPorts = ref([]);
    let showConfig = ref(false)
    let remotePort = ref("")
    let localPort = ref("")
    let waiting = ref(false)

    // 展开行记录
    let expandedRowKeys = ref([])
    let expandedRecord = ref('')

    const bauds = new Map([
      [1200, '1200 bps'],
      [2400, '2400 bps'],
      [4800, '4800 bps'],
      [9600, '9600 bps'],
      [19200, '19200 bps'],
      [38400, '38400 bps'],
      [57600, '57600 bps'],
      [115200, '115200 bps'],
    ]);
    const parities = new Map([
      [78, '无校验(N)'],
      [69, '偶校验(E)'],
      [79, '奇校验(O)'],
    ]);
    const dataBits = new Map([
      [5, 'Data5'],
      [6, 'Data6'],
      [7, 'Data7'],
      [8, 'Data8'],
    ]);
    const stopBits = new Map([
      [1, 'Stop1'],
      [2, 'Stop2'],
    ]);

    watch(
        () => props.peerState,
        (n) => {
          if (n === 7) {
            loadPeers();
          } else {
            peers.value.length = 0
            filterResult.value.length = 0
          }
        }
    );
    watch(filterVal, (n) => {
      filterResult.value.length = 0
      if (n) {
        peers.value.forEach(sss => {
          if (sss.peer_name.indexOf(n) >= 0 || sss.net_addr.indexOf(n) >= 0) {
            filterResult.value.push(sss)
          }
        })
      } else {
        peers.value.forEach(sss => {
          filterResult.value.push(sss)
        })
      }
    });
    onMounted(() => {
      document.oncontextmenu = function () {
        return false;
      };
      window.runtime.EventsOn('peerInfos', (peerInfos) => {
        if (!peerInfos) {
          return
        }
        expandedRecord.value = null
        if(expandedRowKeys.value.length>0){
          peers.value.forEach(sss=>{
            if(sss.peer_mac==expandedRowKeys.value[0]){
              expandedRecord.value = sss
            }
          })
        }
        peers.value.length = 0
        filterResult.value.length = 0
        peerInfos.forEach(sss => {
          sss.key = sss.peer_mac
          if (sss.online) {
            if (sss.dev_type == 'windows') {
              sss.state_img = '/assets/pc_online.png'
            } else {
              sss.state_img = '/assets/router_online.png'
            }
          } else {
            if (sss.dev_type == 'windows') {
              sss.state_img = '/assets/pc_offline.png'
            } else {
              sss.state_img = '/assets/router_offline.png'
            }
          }
          if(expandedRecord.value!=null && sss.peer_mac == expandedRecord.value.peer_mac){
            sss.links = expandedRecord.value.links
          }
          peers.value.push(sss)
          filterResult.value.push(sss)
        })
      });
      loadPeers();
    });
    const loadPeers = () => {
      peers.value.length = 0
      filterResult.value.length = 0
      expandedRowKeys.value.length = 0
      window.go.main.WailsApp.RequestGroupPeers() // 发送请求
    };
    const loadLocalPorts = ()=>{
      window.go.main.SerialApp.GetLocalPortList().then((res)=>{
        localPorts.value.length = 0
        if(res.result){
          localPorts.value= res.list
        }else {
          message.info(res.tip)
        }
      })
      .catch(()=>{
        message.warn("本地串口获取失败")
      })
    };
    const loadConnectedPorts = (record)=>{
      record.connected_ports = reactive([])
      window.go.main.SerialApp.GetConnectedPorts(record.peer_mac).then((res) => {
        if(res.result && res.list && res.list.length>0) {
          record.connected_ports = res.list.map(item=>{
            item.isOpen = true
            return item
          })
        }
      })
      .catch(()=>{
        message.warn("已连接串口获取失败")
      })
    };
    const openSerialPort = (remote,local) => {
      if(!remote){
        message.info("请选择远程串口")
        return
      }
      if(!local){
        message.info("请选择用户串口")
        return
      }
      let  record = expandedRecord.value
      let comB = ""
      localPorts.value.forEach(p=>{
        if(p.comA == local){
          comB = p.comB
        }
      })
      waiting.value = true
      window.go.main.SerialApp.OpenPort(record.peer_mac,remote,local,comB,"").then((res)=>{
        waiting.value = false
        if(res.result){
          message.success(res.tip)
          if(record.connected_ports && record.connected_ports.length>0){
            record.connected_ports.forEach(item => {
              if(item.remote_name == remote){
                item.isOpen = true
              }
            })
          }
        }else {
          message.warn(res.tip)
        }
      })
      .catch(()=>{
        waiting.value = false
        message.warn("串口打开失败")
      })
    };
    const closeSerialPort = (name) => {
      let  record = expandedRecord.value
      waiting.value = true
      window.go.main.SerialApp.ClosePort(record.peer_mac,name).then((res)=>{
        waiting.value = false
        if(res.result){
          message.success(res.tip)
        }else {
          message.warn(res.tip)
        }
        if(record.connected_ports && record.connected_ports.length>0){
          record.connected_ports.forEach(item => {
            if(item.remote_name == name){
              item.isOpen = false
            }
          })
        }
      })
      .catch(()=>{
        waiting.value = false
        message.warn("串口关闭失败")
      })
    };
    const showConfigDialog = (remote,local) =>{
      if(remote){
        // window.go.main.SerialApp.ClosePort(expandedRecord.value.peer_mac,remote)
      }
      remotePort.value = remote
      localPort.value = local
      showConfig.value = true
    }
    const serialDialogClosed = ()=>{
      setTimeout(()=>{
        expandedRecord.value.connected_ports = []
        window.go.main.SerialApp.GetConnectedPorts(expandedRecord.value.peer_mac).then((res) => {
          if(res.result && res.list && res.list.length>0) {
            let ports = res.list.map(item=>{
              item.isOpen = true
              return item
            })
            expandedRecord.value.connected_ports = reactive(ports)
          }
          showConfig.value = false
        })
        .catch(()=>{
          showConfig.value = false
          message.warn("已连接串口获取失败")
        })
      },500)
    }

    const rowClick = (record) => {
      return {
        onClick: () => {
          if (record.online) {
            selectedKeys.value.length = 0
            selectedKeys.value.push(record.peer_mac);
            window.go.main.WailsApp.SelectPeer(record.peer_mac)
          }
        },
      };
    };
    const onSelectChange = (selectedRowKeys, selectedRows) => {
      if (selectedRowKeys.length === 0) return
      if (selectedRows[0].online) {
        selectedKeys.value.length = 0
        selectedKeys.value.push(selectedRowKeys[0]);
        window.go.main.WailsApp.SelectPeer(selectedRowKeys[0])
      }
    };
    const rowExpand = (expanded, record) => {
      if (expanded) {
        expandedRecord.value = record
        if (!record.online) {
          expandedRowKeys.value.length = 0
          return
        }
        expandedRowKeys.value.length = 0 // 只打开一个
        expandedRowKeys.value.push(record.peer_mac)
        loadLocalPorts();
        loadConnectedPorts(record)
        window.go.main.WailsApp.GetLinkInfos(record.peer_mac).then((res) => {
          if (res) {
            res.sort((a, b) => {
              return a.addr - b.addr
            })
            res.forEach(sss => {
              sss.ip = `${(sss.addr >> 24) & 0xFF}.${(sss.addr >> 16) & 0xFF}.${(sss.addr >> 8) & 0xFF}.${sss.addr & 0xFF}`
              sss.receive = sizeFormat(sss.rx)
              sss.send = sizeFormat(sss.tx)
            })
          }
          record.links = res
        })
      } else {
        expandedRowKeys.value.length= 0
        expandedRecord.value = null
      }
    };
    const sizeFormat = (size)=>{
      if(size == 0){
        return '--';
      }
      if(size < 1024) {
        return size.toString() + ' B';
      } else if (size < 1048576) {
        return (size/1024.0).toFixed(2) + ' KB';
      } else if (size < 1073741824) {
        return (size/1048576.0).toFixed(2) + ' MB';
      }else {
        return (size/1073741824.0).toFixed(2) + ' GB';
      }
    };

    const editPeer = (key) => {
      cacheData[key] = cloneDeep(peers.value.filter(item => key === item.key)[0]);
    };
    const savePeer = (key) => {
      window.go.main.WailsApp.UpdateConfig(cacheData[key]).then((res) => {
        if (res.result) {
          Object.assign(peers.value.filter(item => key == item.key)[0], cacheData[key]);
          message.success(res.tip, 3);
        } else {
          message.warn(res.tip, 3);
        }
        delete cacheData[key];
      })
      .catch(() => {
        delete cacheData[key];
      })
    };
    const cancelEditPeer = (key) => {
      delete cacheData[key];
    };
    return {
      columns,
      linkColumns,
      portColumns,
      bauds,
      parities,
      dataBits,
      stopBits,
      peers,
      linkModes,
      filterResult,
      cacheData,
      selectedKeys,
      filterVal,
      editingKey,
      expandedRowKeys,
      expandedRecord,
      localPorts,
      showConfig,
      remotePort,
      localPort,
      waiting,


      loadPeers,
      loadLocalPorts,
      loadConnectedPorts,
      openSerialPort,
      closeSerialPort,
      showConfigDialog,
      serialDialogClosed,
      rowClick,
      rowExpand,
      onSelectChange,
      sizeFormat,
      editPeer,
      savePeer,
      cancelEditPeer,
    };
  },
});
</script>

<style scoped>
.peer-table{
  clear: both;
  background: transparent;
  height: 488px;
  overflow: auto;
}

/*更新*/
:deep(.ant-switch-handle) {
  background: transparent;
}
/*搜索*/
.search-box {
  background: transparent !important;
  margin-right: 30px;
  width: 250px;
  float: right;
  border-bottom: 1px solid #699191d9;
}
:deep(.ant-input){
  background: transparent !important;

}
:deep(.ant-input-affix-wrapper){
  border: 0 ;
  color: white ;
  background: transparent ;
}
:deep(.ant-input-affix-wrapper > input.ant-input){
  border-bottom: 1px solid lightseagreen;
  color: white;
}
:deep(input::-webkit-input-placeholder){
  background: transparent;
}
:deep(.ant-spin-nested-loading){
  height: 100%;
}

/*table*/

/*end table*/

:deep(.ant-empty-normal){
  color: white;
}
:deep(.ant-divider-horizontal.ant-divider-with-text::before){
  border-top-color: white;
}
:deep(.ant-divider-horizontal.ant-divider-with-text::after){
  border-top-color: white;
}
.editable-row-operations a {
  margin-right: 8px;
}
:deep(.ant-card){
  color: white !important;
}
:deep(.ant-card-head){
  color: white !important;
}

:deep(.ant-card-bordered){
  border:0 solid white;
}

:deep(.ant-tabs-tab-btn){
  color: lightgray;
}
:deep(.ant-tabs-tab-active) {
  font-size: larger;
}
</style>

<style scoped>
.peer-table :deep(.ant-table-thead > tr > th)  {
  padding: 5px 5px;
  overflow-wrap: break-word;
  background: transparent;
  color: white;
  border: transparent;
}
.peer-table :deep(.ant-table-tbody > tr > td){
  padding: 5px 5px;
  overflow-wrap: break-word;
  background: transparent;
  color: white;
  border: transparent;
}
.peer-table :deep(.ant-table-tbody .ant-table-row td) {
  padding-top: 8px;
  padding-bottom: 8px;
  color: white;
  border: transparent;
}
.peer-table :deep(.ant-table-placeholder) {
  color: white !important;
  border: transparent;
}
.peer-table :deep(.ant-table-tbody > tr:hover:not(.ant-table-expanded-row):not(.ant-table-row-selected) > td ){
  background: #006aa8f1;
}
.peer-table :deep(.ant-table-tbody > tr:hover > td > div){
  background: transparent;
}
.peer-table :deep(.ant-table-cell > button){
  background: transparent;
}
.peer-table :deep(tr.ant-table-expanded-row > td){
  border: 1px solid lightseagreen;
}
.peer-table :deep(tr.ant-table-expanded-row:hover > td){
  background: transparent;
}

.peer-table :deep(.ant-table-tbody > tr:hover.ant-table-row-selected > td){
  background: #006ac8ff !important;
}
.peer-table :deep(.ant-table-tbody .ant-table-row-selected > td){
  background: #006aa8f1 !important;
}
.peer-table :deep(.ant-table-tbody > tr:hover.ant-table-row-selected > td > div){
  background: #006ac8ff !important;
}
.peer-table :deep(.ant-table-tbody .ant-table-row-selected > td > div){
  background: #006aa8f1 !important;
}

.peer-table :deep(.ant-table-tbody > tr > td.ant-table-cell-row-hover ){
  background: transparent;
}
.peer-table :deep(.ant-table-tbody > tr > td.ant-table-cell-row-hover > div){
  background: transparent;
}
.peer-table :deep(.ant-table-tbody > tr:hover:not(.ant-table-expanded-row):not(.ant-table-row-selected) > td){
  background: transparent;
  color: white;
}

.peer-table :deep(.ant-select-selector){
  background: transparent !important;
  color: white !important;
  border: 1px solid #006ac8ff;
}
</style>
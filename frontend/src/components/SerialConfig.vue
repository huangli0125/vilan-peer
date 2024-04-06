<template>
  <a-tabs style="width: 510px;height: 294px">
    <a-tab-pane key="serial_connect" tab="串口连接管理">
      <a-form :model="serial_config.value" :label-col="labelCol" :wrapper-col="wrapperCol">
        <a-row :gutter="24" class="row-content">
          <a-col :span="12" style="margin-top: 8px">
            <a-form-item label="远程串口" name="remote_name" required>
              <a-select v-model:value="serial_config.value.remote_name" style="width: 130px" @select="RemotePortSelect">
                <a-select-option :value="item.value" v-for="item in remote_ports" :key="item.value">{{item.label}}</a-select-option>
              </a-select>
              <sync-outlined style="margin-left:3px;color: lightseagreen" @click="GetRemotePorts"></sync-outlined>
            </a-form-item>
            <a-form-item label="波特率" name="baud" required>
              <a-select v-model:value="serial_config.value.baud">
                <a-select-option :value="item.value" v-for="item in bauds" :key="item.value">{{item.label}}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="数字位" name="size" required>
              <a-select v-model:value="serial_config.value.size">
                <a-select-option :value="item.value" v-for="item in dataBits" :key="item.value">{{item.label}}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12" style="margin-top: 8px">
            <a-form-item label="用户串口" name="name" required>
              <a-select v-model:value="serial_config.value.name" style="width: 130px">
                <a-select-option :value="item.comA" v-for="item in local_ports" :key="item.index">{{item.comB}}</a-select-option>
              </a-select>
              <sync-outlined style="margin-left:3px;color: lightseagreen" @click="GetLocalPorts"></sync-outlined>
            </a-form-item>
            <a-form-item label="奇偶校验" name="parity" required>
              <a-select v-model:value="serial_config.value.parity">
                <a-select-option :value="item.value" v-for="item in parities" :key="item.value">{{item.label}}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="停止位" name="stop_bits" required>
              <a-select v-model:value="serial_config.value.stop_bits">
                <a-select-option :value="item.value" v-for="item in stopBits" :key="item.value">{{item.label}}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item>
              <div style="text-align: center;">
                <a-button type="primary" @click="SetPortConfig(serial_config.value)" style="display:inline-block;width:150px;">{{btnText}}</a-button>
              </div>
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-tab-pane>
    <a-tab-pane key="virtual_serial" tab="本地虚拟串口管理">
      <a-layout>
        <a-layout-content>
          <a-table
              :columns="columns"
              :data-source="local_ports"
              :pagination="false"
              :locale="{emptyText:' '}"
              :row-key="pair => pair.index"
              style="color:black !important;height: 200px;font-size:13px;overflow: auto">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'operation'">
                <div >
                  <a-popconfirm title="是否删除?" okText="是" cancelText="否" @confirm.stop="DelComPair(record)">
                    <a-tooltip title="删除" color="cyan">
                      <a style="color: red" @click.stop="()=>{}"><MinusCircleOutlined/></a>
                    </a-tooltip>
                  </a-popconfirm>
                </div>
              </template>
            </template>
          </a-table>
        </a-layout-content>
        <a-layout-footer style="text-align: center;padding: 0 10px;">
          <div style="display: flex;">
            <span>中转串口:</span>
            <a-select v-model:value="port_comA" class="com-select">
              <a-select-option :value="item.value" v-for="item in port_list" :key="item.value">{{item.label}}</a-select-option>
            </a-select>
            <span style="margin-left: 10px">用户串口:</span>
            <a-select v-model:value="port_comB" class="com-select">
              <a-select-option :value="item.value" v-for="item in port_list" :key="item.value">{{item.label}}</a-select-option>
            </a-select>
            <a-button type="primary" style="margin-left: 10px" @click="AddComPair">添加</a-button>
          </div>
        </a-layout-footer>
      </a-layout>
    </a-tab-pane>
  </a-tabs>
</template>

<script>
import {defineComponent, onMounted, ref, reactive, getCurrentInstance} from "vue";
import {message} from "ant-design-vue";
import {SyncOutlined,MinusCircleOutlined} from '@ant-design/icons-vue'


const columns = [
  {title: '序号', dataIndex: 'index', width: '15%', key: 'index'},
  {title: '中转串口', dataIndex: 'comA', width: '30%', key: 'comA', align: 'center'},
  {title: '用户串口', dataIndex: 'comB', width: '30%', key: 'comB', align: 'center'},
  {title: '操作', dataIndex: 'operation', key: 'operation', width: '25%', align: 'center',scopedSlots: {customRender: 'operation'}},
];

export default defineComponent( {
  components:{
    SyncOutlined,
    MinusCircleOutlined
  },
  props: {
    peer:{
      default: ''
    },
    remotePort:{
      default: ''
    }
  },
  emits: ["closeDialog"],
  setup(props,{emit}){
    const internalInstance = getCurrentInstance()

    let remote_ports = ref([]);
    let local_ports = ref([]);
    let connected_ports = ref([]);
    let port_list = ref([])
    let port_comA = ref("COM1")
    let port_comB = ref("COM2")

    // 使用reactive时 需要内部再包装一层 才可以使用后台数据更新表单
    // 使用ref 直接替换value
    let serial_config = reactive( { value:{
      name: '',
      baud: 9600,
      parity: 78,
      size: 8,
      stop_bits: 1,
      remote_name: '',
      user_port_name: ''
    }});
    let btnText = ref("设置");
    const bauds = [
      {label:"1200 bps",value:1200},
      {label:"2400 bps",value:2400},
      {label:"4800 bps",value:4800},
      {label:"9600 bps",value:9600},
      {label:"19200 bps",value:19200},
      {label:"38400 bps",value:38400},
      {label:"57600 bps",value:57600},
      {label:"115200 bps",value:115200}
    ];
    const parities =[
      {label:"无校验(N)",value: 78},
      {label:"偶校验(E)",value: 69},
      {label:"奇校验(O)",value: 79}
    ];
    const dataBits = [
      {label:"Data5",value:5},
      {label:"Data6",value:6},
      {label:"Data7",value:7},
      {label:"Data8",value:8}
    ];
    const stopBits = [
      {label:"Stop1",value:1},
      {label:"Stop2",value:2}
    ];
    function GetRemotePorts() {
      window.go.main.SerialApp.GetRemotePortList(props.peer.peer_mac).then((res)=>{
        remote_ports.value.length = 0
        if(res.result){
          res.list.forEach(sss=>{
            remote_ports.value.push({label:sss,value:sss})
          })
        }else {
          message.warn(res.tip)
        }
      })
          .catch(()=>{
            message.warn("远端串口获取失败")
          })
    }
    function GetLocalPorts() {
      window.go.main.SerialApp.GetLocalPortList().then((res)=>{
        local_ports.value.length = 0
        if(res.result){
          local_ports.value= res.list
        }else {
          message.info(res.tip)
        }
      })
      .catch(()=>{
        message.warn("本地串口获取失败")
      })
    }
    function GetPortConfig() {
      window.go.main.SerialApp.GetPortConfig(props.peer.peer_mac,props.remotePort).then((res)=>{
        if(res.result && res.config){
          message.info("参数获取成功")
          let config = res.config
          let old = serial_config.value
          serial_config.value= {
            name: old.name,
            baud: config.baud,
            parity: config.parity,
            size: config.size,
            stop_bits: config.stop_bits,
            remote_name: old.remote_name,
            user_port_name: old.user_port_name
          }
        }else {
          message.warn(res.tip)
        }
      })
      .catch(()=>{
        // message.warn("参数获取失败")
      })
    }
    function SetPortConfig(config) {
      if(!props.remotePort){
        message.warn("没有指定远程串口")
        return
      }
      let conf = JSON.stringify(config)
      window.go.main.SerialApp.SetPortConfig(props.peer.peer_mac,props.remotePort,conf).then((res)=>{
        if(res.result){
          message.info("设置成功")
          ClosePort()
          emit("closeDialog")
        }else {
          message.warn(res.tip)
        }
      })
      .catch(()=>{
        message.warn("串口打开失败")
      })
    }
    function ClosePort() {
      window.go.main.SerialApp.ClosePort(props.peer.peer_mac,props.remotePort)
    }
    function AddComPair() {
      window.go.main.SerialApp.AddLocalPort(port_comA.value,port_comB.value).then((res)=>{
        if(res.result){
          message.info(res.tip)
        }else {
          message.warn(res.tip)
        }
        GetLocalPorts()
      })
    }
    function DelComPair(record) {
      window.go.main.SerialApp.DelLocalPort(record.index).then((res)=>{
        if(res.result){
          message.info(res.tip)
        }else {
          message.warn(res.tip)
        }
        GetLocalPorts()
      })
    }
    onMounted(()=>{
      serial_config.value.remote_name = props.remotePort
      GetPortConfig()
      for (let i=1;i<100;i++){
        port_list.value.push({label:`COM${i}`,value:`COM${i}`})
      }
      GetRemotePorts();
      GetLocalPorts();
    });
    return {
      internalInstance,
      labelCol: {
        span: 8,
      },
      wrapperCol: {
        span: 16,
      },
      columns,
      btnText,
      bauds,
      parities,
      dataBits,
      stopBits,
      serial_config,
      local_ports,
      connected_ports,
      port_comA,
      port_comB,
      port_list,

      GetRemotePorts,
      GetLocalPorts,
      GetPortConfig,
      SetPortConfig,
      ClosePort,
      AddComPair,
      DelComPair,
    };
  },
})
</script>

<style scoped>
.row-content{
  margin: 0 0 !important;
  height: 100%;
}
.com-select{
  height: 30px;
  width: 100px;
}
:deep(.ant-table-thead > tr > th)  {
  padding: 5px 5px;
  overflow-wrap: break-word;
}
:deep(.ant-table-tbody > tr > td){
  padding: 5px 5px;
  overflow-wrap: break-word;
}
:deep(.ant-table-tbody .ant-table-row td) {
  padding-top: 8px;
  padding-bottom: 8px;
}

:deep(.ant-select-single:not(.ant-select-customize-input) .ant-select-selector){
  height: 30px;
}


</style>
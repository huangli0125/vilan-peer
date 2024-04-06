<template>
  <a-layout style="background: transparent">
    <a-layout  style="background: transparent">
      <a-layout-sider :collapsed="true" :trigger="null" collapsible class="side-layout">
        <a-menu v-model:selectedKeys="selectedKeys" theme="dark" mode="inline" @click="handleMenuClick">
          <a-menu-item key="main" title="主页">
            <appstore-outlined />
          </a-menu-item>
          <a-menu-item  key="setting" title="设置" >
            <setting-outlined />
          </a-menu-item>
          <a-menu-item key="log" title="日志" >
            <profile-outlined />
          </a-menu-item>
        </a-menu>
      </a-layout-sider>
      <a-layout class="content-layout" style="background: transparent">
        <PeerView v-if="selectedKeys.indexOf('main')>=0" :peer-state="peerState"></PeerView>
        <Setting v-if="selectedKeys.indexOf('setting')>=0"  @configChanged="configChanged"></Setting>
        <log v-if="selectedKeys.indexOf('log')>=0"></log>
      </a-layout>
    </a-layout>
    <a-layout-footer  style="background: #003366;margin-top:5px;padding:5px;height: 30px">
      <a-row style="color: #aac1dc;text-align: left;font-size: 12px">
        <a-col :span="2">当前状态:</a-col>
        <a-col v-if="peerState<7" :span="3" style="color: orangered">{{ peerStateStr }}</a-col>
        <a-col v-else :span="3" style="color:limegreen">{{ peerStateStr }}</a-col>
        <a-col :span="4" :offset="7" style="text-align: right;">
          <div :title="config.peerName" style="text-align: right;white-space:nowrap;overflow: hidden">
            {{ config.peerName }}
          </div>
        </a-col>
        <a-col :span="3" style="text-align: center" >{{ config.ipAddr }}</a-col>
        <a-col :span="5" style="overflow: hidden">
          <a-tooltip color="transparent"  >
            Tx/Rx: {{ sizeFormat(statsInfo.Tx) }} / {{ sizeFormat(statsInfo.Rx) }}
            <template v-slot:title >
              <div style="font-size: 12px;white-space:nowrap;overflow: hidden">
                P2P Tx/Rx: {{ sizeFormat(statsInfo.P2pTx) }} / {{ sizeFormat(statsInfo.P2pRx) }}
              </div>
            </template>
          </a-tooltip>
        </a-col>
      </a-row>
    </a-layout-footer>
  </a-layout>
</template>
<script>

import {defineComponent, onMounted, reactive, ref} from 'vue';
import { AppstoreOutlined, SettingOutlined,ProfileOutlined } from '@ant-design/icons-vue';
import PeerView from "@/components/PeerView";
import Setting from "@/components/Setting";
import Log from "@/components/Log";

export default defineComponent({
  components:{
    PeerView,
    Setting,
    Log,
    ProfileOutlined,
    AppstoreOutlined,
    SettingOutlined,
  },
  setup() {
    let selectedKeys = ref(['main']);
    const stateGroup=['未初始化','初始化错误','初始化成功','网络未连接','网络已连接','服务未注册','服务注册失败','服务正常运行'];
    let  peerState = ref(0);
    let  peerStateStr = ref('未初始化')
    let  statsInfo=reactive({Rx:0,Tx:0,P2pRx:0,P2pTx:0});
    let  config = ref({})
    onMounted(()=>{
      document.oncontextmenu = function () {
        return false;
      };
      window.go.main.WailsApp.GetPeerState().then((state)=>{
        peerState.value = state
      })
      window.runtime.EventsOn('peerState', peerStateChange);
      window.runtime.EventsOn('statistic', statistic);
      window.go.main.WailsApp.GetConfig().then((res)=>{
        if(res){
          config.value.peerName = res.peer_name
          config.value.ipAddr = res.ip_addr
        }
      });
    });
    const handleMenuClick=(event)=>{
      if(selectedKeys.value.indexOf(event.key)>=0){
        return
      }
      switch (event.key) {
        case 'main':
          break
        case 'setting':
          break
        case 'log':
          break
      }
    };
    const peerStateChange=(state)=>{
      peerState.value = state
      peerStateStr.value = stateGroup[state]
      if(state === 7){
        window.go.main.WailsApp.GetConfig().then((res)=>{
          if(res){
            config.value.peerName = res.peer_name
            config.value.ipAddr = res.ip_addr
          }
        });
      }
    };
    const configChanged=(conf)=>{
      config.value.peerName = conf.peer_name
      config.value.ipAddr = conf.ip_addr
    };
    const statistic=(stats)=>{
      if(stats.p2p_send) {
        statsInfo.Tx = stats.trans_send+stats.p2p_send
      }else{
        statsInfo.Tx = stats.trans_send
      }
      if(stats.p2p_receive){
        statsInfo.Rx = stats.trans_receive+stats.p2p_receive
      }else {
        statsInfo.Rx = stats.trans_receive
      }
      statsInfo.P2pTx = stats.p2p_send
      statsInfo.P2pRx = stats.p2p_receive
    };
    const sizeFormat=(size)=>{
      if(!size){
        return '0 B'
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
    return {
      selectedKeys,
      stateGroup,
      peerState,
      peerStateStr,
      statsInfo,
      config,

      handleMenuClick,
      peerStateChange,
      configChanged,
      statistic,
      sizeFormat,
    };
  },
});
</script>
<style>
.side-layout {
  background: #003366;
  height: 529px;
  margin-top: 5px;
}
.side-layout div {
  background: #003366;
}
.side-layout ul{
  background: #003366 !important;
}
.side-layout ul li{
  background: #003366;
}
.content-layout div{
  background: #064468f1;
}

</style>
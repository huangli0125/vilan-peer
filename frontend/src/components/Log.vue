<template>
  <div>
    <div style="margin:10px 20px; font-size: 14px;color: white">
      <a-row :gutter="16">
        <a-col :span="2" style="padding-top: 10px">等级</a-col>
        <a-col :span="4" style="padding-top: 10px" >时间</a-col>
        <a-col :span="10" style="padding-top: 10px">内容</a-col>
        <a-col :span="8" style="text-align: right">
          <a-select v-model:value="filterLog" :options="logOptions" allow-clear @change="handleChange" class="log-filter"></a-select>
          <a-tooltip title="清空">
            <a-button  type="primary" danger shape="circle" size="middle" @click="clearLogs" >
              <template #icon><ClearOutlined /></template>
            </a-button>
          </a-tooltip>
        </a-col>
      </a-row>
    </div>
    <a-list size="small" bordered :data-source="filterResult" class="log-list">
      <template #renderItem="{ item }">
        <a-list-item style="border: 0;" :style="{'color': item.color}">
          <a-row style="width: 100%" :gutter="16">
            <a-col :span="2">{{ logLevels[item.logType] }}</a-col>
            <a-col :span="4">{{ item.time }}</a-col>
            <a-col :span="18" style="overflow: hidden">{{ item.content }}</a-col>
          </a-row>
        </a-list-item>
      </template>
    </a-list>
  </div>
</template>

<script>
import {defineComponent, onMounted, ref} from 'vue';
import { ClearOutlined} from '@ant-design/icons-vue'

const logLevels = ['错误','警告','信息','调试'];
const logOptions = [{value:0,label:"错误"},
  {value:1,label:"警告"},
  {value:2,label:"信息"},
  {value:3,label:"调试"}];

export default defineComponent({
  name:"Log",
  components:{
    ClearOutlined
  },
  setup() {
    const logs = ref([]);
    const filterResult = ref([]);
    const filterLog = ref(null);

    onMounted(()=>{
      logs.value.length =0;
      filterResult.value.length = 0
      window.go.main.WailsApp.GetHisLog().then((res)=>{
        if(res &&res.length>0){
          res.forEach(sss=>{
            addMsgColor(sss)
            logs.value.push(sss)
            filterResult.value.push(sss)
          })
        }
      })
      window.runtime.EventsOn('log', onLog);
    });
    const addMsgColor = (msg) => {
      switch (msg.logType) {
        case 0:
          msg.color = '#F56C6C'
          break
        case 1:
          msg.color = '#E6A23C'
          break;
        case 2:
          msg.color = 'white'
          break;
        default:
          msg.color = '#909399'
          break
      }
    }
    const onLog = (msg)=>{
      addMsgColor(msg)
      logs.value.unshift(msg)
      if(filterLog.value || filterLog.value===0){
        if(msg.logType == filterLog.value){
          filterResult.value.unshift(msg)
        }
      }else{
        filterResult.value.unshift(msg)
      }
      if(filterResult.value.length>100) {
        filterResult.value.pop()
      }
      if(logs.value.length>100) {
        logs.value.pop()
      }
    };
    const handleChange = (value) => {
      filterResult.value.length = 0
      if(value || value===0){
        logs.value.forEach(sss=>{
          if(sss.logType == value){
            filterResult.value.push(sss)
          }
        })
      }else {
        logs.value.forEach(sss=>{
          filterResult.value.push(sss)
        })
      }
    };
    const clearLogs=()=>{
      logs.value.length =0;
      filterResult.value.length = 0
      window.go.main.WailsApp.ClearHisLog()
    }
    return {
      logLevels,
      logOptions,
      logs,
      filterResult,
      filterLog,

      addMsgColor,
      onLog,
      handleChange,
      clearLogs
    };
  },
});
</script>

<style scoped>
.log-list{
  height: 471px;
  margin: 10px 5px;
  color: white;
  border: 1px solid lightseagreen;
  overflow: auto;
}
.log-filter{
  background: transparent;
  color: white;
  text-align: center;
  width: 100px;
  margin-right: 15px;
}

:deep(.ant-list-header){
  border-bottom: 1px solid lightseagreen !important;
}
:deep(.ant-select-selector){
  background: transparent !important;
  border: 1px solid lightseagreen !important;
}
:deep(.ant-select-clear){
  border-radius: 6px;
  color: lightseagreen;
}
:deep(.ant-select-clear:hover){
  color: royalblue;
}
:deep(.ant-select:not(.ant-select-customize-input)){
  border: 0;
}
:deep(.ant-select-arrow svg){
 color: white;
}
</style>
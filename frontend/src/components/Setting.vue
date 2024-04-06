<template>
  <a-form ref="form" :model="formState" :rules="rules"  :label-col="labelCol" :wrapper-col="wrapperCol" class="form-content" autoComplete="off" @finish="onSubmit">
    <a-row :gutter="24" class="row-content">
      <a-col :span="12" style="margin-top: 8px">
        <a-form-item label="终端名称" name="peer_name">
          <a-input v-model:value="formState.peer_name" />
        </a-form-item>
        <a-form-item label="终端地址" name="ip_addr">
          <a-input :disabled="formState.ip_mode===1" v-model:value="formState.ip_addr" />
        </a-form-item>
        <a-form-item label="网络组名称" name="group_name">
          <a-input v-model:value="formState.group_name" />
        </a-form-item>
        <a-form-item label="传输加密" name="crypt_type">
          <a-select v-model:value="formState.crypt_type" :options="crypt_types" />
        </a-form-item>
        <a-form-item label="服务地址" name="server_ip">
          <a-input v-model:value="formState.server_ip" />
        </a-form-item>
        <a-form-item label="虚拟网卡名称" name="tab_name">
          <a-input :disabled="true" v-model:value="formState.tab_name" />
        </a-form-item>
        <a-form-item label="打印日志" name="enable_log">
          <a-switch v-model:checked="formState.enable_log" />
        </a-form-item>

      </a-col>

      <a-col :span="12" style="margin-top: 8px">
        <a-form-item label="IP地址模式" name="ip_mode">
          <a-radio-group v-model:value="formState.ip_mode">
            <a-radio :value="1">自动分配</a-radio>
            <a-radio :value="2">静态地址</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item  label="子网掩码" name="ip_mask">
          <a-input :disabled="formState.ip_mode===1" v-model:value="formState.ip_mask" />
        </a-form-item>
        <a-form-item label="网络组密码" name="group_pwd">
          <a-input-password v-model:value.trim="formState.group_pwd" />
        </a-form-item>
        <a-form-item label="传输密码" name="peer_pwd">
          <a-input-password v-model:value.trim="formState.peer_pwd" />
        </a-form-item>
        <a-form-item label="服务端口" name="server_port">
          <a-input v-model:value.number="formState.server_port" />
        </a-form-item>
        <a-form-item label="虚拟网卡MAC" name="hw_mac">
          <a-input :disabled="true" v-model:value="formState.hw_mac" />
        </a-form-item>
        <a-form-item label="日志级别" name="log_level">
          <a-select v-model:value="formState.log_level" :options="logLevels" />
        </a-form-item>
      </a-col>
      <a-form-item style="margin-left: 320px">
        <a-button type="primary" html-type="submit" style="width: 180px" :loading="saving">{{saving?'保存配置中...':'保    存'}}</a-button>
      </a-form-item>
    </a-row>

  </a-form>
</template>
<script>
import { message } from 'ant-design-vue';
import {defineComponent, onMounted, reactive, ref, toRaw} from 'vue';
import Validator from '@/common/Validator'

export default defineComponent({
  emits: ["configChanged"],
  setup(_,{emit}) {
    const formState = reactive({
      peer_name:'',
      ip_mode:1,
      ip_addr:'',
      ip_mask:'',
      group_name:'',
      group_pwd:'',
      crypt_type:0,
      peer_pwd:'',
      server_ip:'',
      server_port:0,
      tab_name:'',
      hw_mac:'',
      enable_log:false,
      log_level:0,
    });
    const logLevels = [
      {label:"调试信息",value:3},
      {label:"普通信息",value:2},
      {label:"告警信息",value:1},
      {label:"错误信息",value:0}
    ];
    const crypt_types = [
      {label:"不加密",value:0},
      {label:"加密",value:1}
      // {label:"DES加密",value:2},
      // {label:"RSA加密",value:3}
    ];
    let saving = ref(false)

    onMounted(()=>{
      document.oncontextmenu = function () {
        return false;
      };
      loadConfig()
    });

    const validatorName = (rule, value) => {
      if (value === '') {
        return Promise.reject('请输入终端名称')
      }else if(strLen(value)>50) {
        return Promise.reject('终端名称长度不能大于50')
      } else {
        return Promise.resolve()
      }
    };
    const validatorGroup = (rule, value) => {
      if (value === '') {
        return Promise.reject('请输入网络组名称')
      } else if(strLen(value) > 50) {
        return Promise.reject('网络组名称长度不能大于50')
      }else {
        return Promise.resolve()
      }
    };
    const validatorPassword = (rule, value) => {
      if (value === '') {
        return Promise.reject('请输入密码')
      } else {
        return Promise.resolve()
      }
    };
    const validateMac = (rule, value) => {
      if (value === '') {
        return Promise.reject('请输入MAC')
      } else if (!Validator.macaddr(value)) {
        return Promise.reject('请输入正确的MAC')
      }
      else {
        return Promise.resolve()
      }
    };
    const validateIP = (rule, value) => {
      if (value === '') {
        return Promise.reject('请输入IP地址')
      } else if (!Validator.ip4addr(value)) {
        return Promise.reject('请输入正确的IP地址')
      } else {
        return Promise.resolve()
      }
    };
    const validateMask = (rule, value) => {
      if (value === '') {
        return Promise.reject('请输入子网掩码')
      } else if (!Validator.netmask4(value)) {
        return Promise.reject('请输入正确的子网掩码')
      } else {
        return Promise.resolve()
      }
    };
    const validatePort = (rule, value) => {
      if (value === '') {
        return Promise.reject('请输入端口号')
      } else if (!Validator.uinteger(value)) {
        return Promise.reject('端口号须为正整数')
      } else if(parseInt(value) > 65535){
        return Promise.reject('端口号不能大于65535')
      }
      else {
        return Promise.resolve()
      }
    };
    const rules= {
          peer_name:[{ validator: validatorName }],
          group_name:[{ validator: validatorGroup }],
          // group_pwd:[{ validator: validatorPassword }],
          peer_pwd:[{ validator: validatorPassword }],
          ip_addr: [{ validator: validateIP }],
          ip_mask: [{ validator: validateMask }],
          server_ip: [{ validator: validateIP }],
          hw_mac:[{ validator: validateMac }],
          server_port:[{validator: validatePort}]
    };
    const loadConfig=()=>{
      window.go.main.WailsApp.GetConfig().then((res)=>{
        if(res){
          Object.keys(res).forEach((key)=>{
            formState[key] = res[key];
          });
          emit('configChanged',toRaw(formState))
        }
      })
    };
    const onSubmit=()=>{
      saving.value = true
      window.go.main.WailsApp.SaveConfig(toRaw(formState)).then((res)=>{
        saving.value = false
        if(res && res.indexOf("成功")>=0){
          message.success(res,3);
          emit('configChanged',toRaw(formState))
        }else{
          saving.value = false
          message.warn("配置修改失败:"+res,3);
        }
      })
    };
    const strLen=(str)=>{
      let len = 0;
      for (let i=0; i<str.length; i++) {
        let c = str.charCodeAt(i);
        //单字节加1
        if ((c >= 0x0001 && c <= 0x007e) || (0xff60<=c && c<=0xff9f)) {
          len++;
        }
        else {
          len+=2;
        }
      }
      return len;
    };
    return {
      labelCol: {
        span: 8,
      },
      wrapperCol: {
        span: 16,
      },
      logLevels,
      crypt_types,
      formState,
      rules,
      saving,

      loadConfig,
      onSubmit,
      strLen,
    };
  },
});
</script>

<style scoped>
.form-content{
  height: 534px;
  padding: 10px 10px 0px 5px;
}
.row-content{
  margin: 0 0 !important;
  height: 100%;
}
:deep(.ant-form-item-label > label){
  color: whitesmoke;
}
:deep(label){
  color: white;
}
:deep(input ){
  background: left;
  color: white;
}
:deep(.ant-input ){
  background: left !important;
  color: white;
}
:deep(.ant-form-item-control-input .ant-select-selector){
  background: left !important;
  color: white !important;
}
:deep(.ant-form-item-control-input .ant-input-password){
  background: left !important;
  color: white !important;
}
:deep(.ant-form-item-control-input svg){
  color: white !important;
}
:deep(.ant-switch-handle){
  background: transparent;
}
:deep(.ant-input[disabled]){
  background: left;
  color: white;
}
</style>
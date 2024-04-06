<template>
  <div class="header">
    <div style="height: 30px;width: 100%;display: flex;justify-content: right;background: #003366;--wails-draggable: drag"  data-wails-drag>
      <div v-if="platform != 'darwin'" style="display: inline-flex;width:200px; padding-top: 5px;padding-left: 5px; margin-right:calc(100% - 310px);font-size:14px;color: #1890FF;text-align: left">{{title}}</div>
      <div v-if="platform != 'darwin'" data-wails-no-drag style="display: inline-flex" :style="platform == 'linux' ? 'margin-left:calc(100% - 105px);' : ''">
        <div class="MinorMax" style="width: 35px;height: 25px;display: flex;justify-content: center;align-items: center;--wails-draggable:none;"
             @click="minWindow">
          <LineOutlined style="color: aliceblue;font-size: 15px"/>
        </div>
        <div  style="width: 35px;height: 25px;display: flex;justify-content: center;align-items: center;--wails-draggable:none;"
             @click="maxWindow">
          <RetweetOutlined v-if="maxStatus"  style="color: gray;font-size: 12px"/>
          <BorderOutlined v-else style="color: gray;font-size: 12px"/>
        </div>
        <div class="close"
             style="width: 35px;height: 25px;display: flex;justify-content: center;align-items: center;border-radius:0 6px 0 0;--wails-draggable:none;"
             @click="closeWindow">
          <CloseOutlined style="color: aliceblue;font-size: 15px"/>
        </div>
      </div>
    </div>
    <Home/>
  </div>
</template>

<script>
import {BorderOutlined, CloseOutlined, LineOutlined,RetweetOutlined} from "@ant-design/icons-vue";
import Home from "@/components/Home.vue";
import {defineComponent, onMounted, ref} from 'vue';
import {WindowHide} from "../wailsjs/runtime";


export default defineComponent({
  components: {
    CloseOutlined,
    BorderOutlined,
    LineOutlined,
    RetweetOutlined,
    Home
  },
  setup(){
    const title = ref("Vlian客户端")
    let maxStatus = ref(false)
    let platform = ref("windows")
    onMounted(()=>{
      document.oncontextmenu = function () {
        return false;
      };
      window.go.main.WailsApp.PullPlatform().then(res => {
        platform.value = res;
      });
    });
    function getPlatform() {
      return platform;
    }
    function minWindow() {
      //window.runtime.WindowMinimise();
      window.go.main.WailsApp.HideWindow();
    }
    function maxWindow() {
      // if (this.maxStatus){
      //   window.runtime.WindowUnmaximise();
      //   this.maxStatus = false;
      // }else{
      //   window.runtime.WindowMaximise();
      //   this.maxStatus = true;
      // }
    }
    function closeWindow() {
      window.runtime.Quit();
    }
    return {
      title,
      maxStatus,
      platform,

      getPlatform,
      minWindow,
      maxWindow,
      closeWindow
    }
  }
});

</script>
<style>
html, body, #app {
  font-size: 30px;
  margin: 0;
  padding: 0;
  height: 100%;
  width: 100%;
  background-color: rgba(255, 255, 255, 0);
  font-family: Avenir, Helvetica, Arial, sans-serif;
}

.header {
  height: 100%;
  width: 100%;
  background: #064468f1;
  border-radius: 0;
  border: 0;
  overflow-x: hidden;
}

.MinorMax:hover {
  background: #006aa8f1;
}

.close:hover {
  background: #b20000f1;
}
/*弹窗样式*/
.ant-modal-header {
  padding:5px 20px !important;
  background: #0F8FDF !important;
}
.ant-modal-title{
  color: white !important;
  font-size: 16px !important;
}
.ant-modal-close-x {
  width: 36px !important;
  line-height: 28px !important;
  color: red !important;
}
.ant-modal-body{
  padding: 8px 24px !important;
}

/*滚动条样式*/
::-webkit-scrollbar {
  width: 4px;
}
::-webkit-scrollbar-thumb {
  border-radius: 10px;
  box-shadow: inset 0 0 5px rgb(24, 144, 255);
  background: rgba(0,0,0,0.2);
}
::-webkit-scrollbar-track {
  box-shadow: inset 0 0 5px rgba(0,0,0,0.2);
  border-radius: 0;
  background: rgba(0,0,0,0.1);
}
</style>

## serial2net

---
### 此模块实现串口的远程调用功能，可独立出来使用。
* 主要功能为： 
* 串口基本的操作：打开关闭、读写、参数设置
* 获取串口列表
* 读写状态反馈
* 支持单个连接对多个串口读写

#### 远程串口交互流程为
1. 与远端建立TCP连接，为串口交互提供链路
2. 本地建立通过com0com虚拟串口驱动创建配对虚拟串口,其中一个串口(comA)供用户使用,另一个(comB)程序内部使用,两个串口是直接连通的
3. 打开本地串口comB,并发布打开远端串口命令
4. 当两边的串口都打开成功后，用户可以打开串口comA进行数据收发
5. comA的数据经由comB通过TCP连接发送远端串口
6. 收到远端的数据写入comB，用户端就可以收到这些数据

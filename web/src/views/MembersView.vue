<script setup lang="ts">
import { Check, ShieldCheck, UserMinus, Users, X } from "@lucide/vue";
import { computed, onMounted, ref } from "vue";
import { approveMember, changeMemberRole, disableMember, listMembers, rejectMember, type MemberData, type MemberRole } from "@/api";
import { useSessionStore } from "@/stores/session";

const session=useSessionStore();const members=ref<MemberData[]>([]);const loading=ref(true);const error=ref("");
const pending=computed(()=>members.value.filter(v=>v.status==="PENDING"));const active=computed(()=>members.value.filter(v=>v.status==="ACTIVE"));
async function load(){loading.value=true;error.value="";try{members.value=await listMembers()}catch(e){error.value=e instanceof Error?e.message:"读取成员失败"}finally{loading.value=false}}
async function act(fn:()=>Promise<unknown>){try{await fn();await load()}catch(e){error.value=e instanceof Error?e.message:"操作失败"}}
const approve=(m:MemberData)=>act(()=>approveMember(m.id,m.version));const reject=(m:MemberData)=>act(()=>rejectMember(m.id,m.version));
const role=(m:MemberData,r:MemberRole)=>act(()=>changeMemberRole(m.id,m.version,r));const disable=(m:MemberData)=>act(()=>disableMember(m.id,m.version));
onMounted(load);
</script>

<template><div class="page-stack"><section class="inventory-hero"><div><p class="eyebrow">FAMILY MEMBERS</p><h1>家庭成员</h1><p>审批加入申请，管理家庭成员的角色与状态。</p></div><Users :size="42" /></section>
<p v-if="error" class="form-message error">{{error}}</p><section v-if="session.memberRole!=='OWNER'" class="pixel-panel coming-soon-inline"><ShieldCheck/><h3>仅家庭所有者可以管理成员</h3></section>
<template v-else><section class="pixel-panel member-section"><header><h2>待审批申请</h2><span class="count-badge">{{pending.length}}</span></header><div v-if="!pending.length" class="inventory-empty"><span>🌱</span><p>暂时没有新的申请</p></div><article v-for="m in pending" :key="m.id" class="member-row"><span class="avatar">{{m.display_name.slice(0,1)}}</span><div><strong>{{m.display_name}}</strong><small>@{{m.username}} · 等待审批</small></div><div class="member-actions"><button class="secondary-button" @click="reject(m)"><X :size="15"/>拒绝</button><button class="primary-pixel-button compact" @click="approve(m)"><Check :size="15"/>批准</button></div></article></section>
<section class="pixel-panel member-section"><header><h2>当前成员</h2><span class="count-badge">{{active.length}}</span></header><p v-if="loading">正在读取…</p><article v-for="m in active" :key="m.id" class="member-row"><span class="avatar">{{m.display_name.slice(0,1)}}</span><div><strong>{{m.display_name}}</strong><small>@{{m.username}} · {{m.role}}</small></div><div v-if="m.role!=='OWNER'" class="member-actions"><button class="secondary-button" @click="role(m,m.role==='ADMIN'?'MEMBER':'ADMIN')">{{m.role==='ADMIN'?'降为成员':'设为管理员'}}</button><button class="secondary-button danger-text" @click="disable(m)"><UserMinus :size="15"/>禁用</button></div></article></section></template></div></template>

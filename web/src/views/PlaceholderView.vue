<script setup lang="ts">
import { ArrowLeft, Hammer, Sprout } from "@lucide/vue";
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";

const route = useRoute();
const router = useRouter();

const content = computed(() => {
  const section = String(route.params.section);
  const pages: Record<string, { emoji: string; title: string; description: string }> = {
    memos: { emoji: "📝", title: "家庭备忘", description: "私人备忘、公共提醒和消息通知将在第二阶段开放。" },
    knowledge: { emoji: "📚", title: "家庭知识库", description: "这里将收藏家庭文档、说明书与生活经验，第四阶段开放。" },
    cameras: { emoji: "📹", title: "家庭监控", description: "RTSP 视频将在媒体网关接入后显示在这里，第五阶段开放。" },
    devices: { emoji: "💡", title: "智能家居", description: "这里将展示 Home Assistant 中的设备和实时状态，第五阶段开放。" },
    agent: { emoji: "🤖", title: "家庭 Agent", description: "家庭知识问答与结构化工具调用将在第三阶段开放。" },
    members: { emoji: "👨‍👩‍👧", title: "家庭成员", description: "成员、角色和权限管理是第一阶段的下一项工作。" },
    settings: { emoji: "🏡", title: "家庭设置", description: "家庭创建后只在这里修改名称、图标和基础资料，不再提供创建入口。" },
    profile: { emoji: "🌻", title: "个人资料", description: "这里将管理你的昵称、头像和通知偏好。" },
  };
  return pages[section] ?? { emoji: "🌱", title: "新的家庭空间", description: "这片空间还在准备中。" };
});
</script>

<template>
  <section class="placeholder-page pixel-panel">
    <div class="placeholder-decoration"><Sprout :size="28" /></div>
    <span class="placeholder-emoji">{{ content.emoji }}</span>
    <p class="eyebrow"><Hammer :size="15" /> COMING SOON</p>
    <h1>{{ content.title }}</h1>
    <p>{{ content.description }}</p>
    <button class="secondary-button" type="button" @click="router.push('/app')">
      <ArrowLeft :size="17" /> 返回家庭首页
    </button>
  </section>
</template>

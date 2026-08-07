<script setup lang="ts">
import { BadgeCheck, LockKeyhole, Save, UserRound } from "@lucide/vue";
import { onMounted, ref } from "vue";

import { getProfile, updateProfile, type MemberAvatar, type MemberData, type PresenceStatus } from "@/api";
import { useSessionStore } from "@/stores/session";

const session = useSessionStore();
const profile = ref<MemberData | null>(null);
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const success = ref("");
const form = ref<{ displayName: string; avatarKey: MemberAvatar; presenceStatus: PresenceStatus | "" }>({
  displayName: "", avatarKey: "initials", presenceStatus: "",
});

const avatars: Array<{ value: MemberAvatar; emoji: string; label: string }> = [
  { value: "initials", emoji: "", label: "姓氏头像" },
  { value: "man", emoji: "👨", label: "成年男性" },
  { value: "woman", emoji: "👩", label: "成年女性" },
  { value: "boy", emoji: "👦", label: "男孩" },
  { value: "girl", emoji: "👧", label: "女孩" },
  { value: "dog", emoji: "🐶", label: "小狗" },
];

function avatarDisplay(value: MemberAvatar) {
  if (value === "initials") return form.value.displayName.trim().slice(0, 1) || session.initials;
  return avatars.find((item) => item.value === value)?.emoji ?? session.initials;
}

const statuses: Array<{ value: PresenceStatus | ""; emoji: string; label: string }> = [
  { value: "", emoji: "🌿", label: "未设置" },
  { value: "HOME", emoji: "🏡", label: "在家" },
  { value: "SCHOOL", emoji: "🏫", label: "在校" },
  { value: "WORKING", emoji: "💻", label: "工作ing" },
  { value: "OUT", emoji: "🚲", label: "外出" },
  { value: "NAPPING", emoji: "😴", label: "午睡" },
  { value: "RESTING", emoji: "☕", label: "休息ing" },
  { value: "SICK", emoji: "🤒", label: "生病" },
  { value: "STUDYING", emoji: "📖", label: "学习ing" },
];

function applyProfile(value: MemberData) {
  profile.value = value;
  form.value = {
    displayName: value.display_name,
    avatarKey: value.avatar_key || "initials",
    presenceStatus: value.presence_status || "",
  };
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    applyProfile(await getProfile());
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取个人资料失败";
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!profile.value || saving.value) return;
  saving.value = true;
  error.value = "";
  success.value = "";
  try {
    const updated = await updateProfile({
      display_name: form.value.displayName.trim(),
      avatar_key: form.value.avatarKey,
      presence_status: form.value.presenceStatus,
      version: profile.value.version,
    });
    applyProfile(updated);
    await session.refreshSession();
    success.value = "个人资料已经保存";
    window.dispatchEvent(new Event("vhome:dashboard-changed"));
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "保存个人资料失败";
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="page-stack profile-page">
    <section class="inventory-hero profile-hero">
      <div>
        <p class="eyebrow">MY PROFILE</p>
        <h1>个人资料</h1>
        <p>选择你在家庭看板中的昵称、头像和当前状态。</p>
      </div>
      <span class="profile-hero-avatar">{{ avatarDisplay(form.avatarKey) }}</span>
    </section>

    <p v-if="error" class="form-message error">{{ error }}</p>
    <p v-if="success" class="form-message success">{{ success }}</p>
    <section v-if="loading" class="pixel-panel profile-card">正在读取个人资料…</section>

    <form v-else-if="profile" class="pixel-panel profile-card material-form" @submit.prevent="save">
      <header>
        <div class="settings-card-icon"><UserRound /></div>
        <div><h2>展示资料</h2><p>修改后会同步显示在首页、侧边栏和右上角。</p></div>
      </header>

      <div class="profile-form-body">
        <label>
          <span>昵称</span>
          <input v-model="form.displayName" required maxlength="64" />
          <small class="form-help">登录账号 {{ profile.username }} 不会随昵称变化。</small>
        </label>

        <fieldset class="profile-choice-field">
          <legend>头像</legend>
          <div class="profile-avatar-grid">
            <button
              v-for="avatar in avatars" :key="avatar.value" type="button"
              class="profile-avatar-choice" :class="{ selected: form.avatarKey === avatar.value }"
              @click="form.avatarKey = avatar.value"
            >
              <span>{{ avatarDisplay(avatar.value) }}</span><small>{{ avatar.label }}</small>
              <BadgeCheck v-if="form.avatarKey === avatar.value" :size="15" />
            </button>
          </div>
        </fieldset>

        <fieldset class="profile-choice-field">
          <legend>当前状态</legend>
          <div class="profile-status-grid">
            <button
              v-for="status in statuses" :key="status.value" type="button"
              class="profile-status-choice" :class="{ selected: form.presenceStatus === status.value }"
              @click="form.presenceStatus = status.value"
            >
              <span>{{ status.emoji }}</span>{{ status.label }}
            </button>
          </div>
        </fieldset>

        <button class="primary-pixel-button compact profile-save" :disabled="saving">
          <Save :size="17" /> {{ saving ? "保存中…" : "保存个人资料" }}
        </button>
      </div>
    </form>

    <section class="pixel-panel profile-card profile-coming-soon">
      <LockKeyhole :size="22" />
      <div><h2>更多个人配置</h2><p>密码、通知偏好等复杂设置当前暂不开放。</p></div>
    </section>
  </div>
</template>

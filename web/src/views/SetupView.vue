<script setup lang="ts">
import { Home, Sparkles } from "@lucide/vue";
import { ref } from "vue";
import { useRouter } from "vue-router";

import { useSessionStore } from "@/stores/session";

const router = useRouter();
const session = useSessionStore();

const householdName = ref("");
const householdPassword = ref("");
const ownerName = ref("");
const ownerPassword = ref("");
const isLoading = ref(false);
const errorMessage = ref("");

async function createHousehold() {
  if (
    !householdName.value.trim() ||
    householdPassword.value.length < 10 ||
    !ownerName.value.trim() ||
    ownerPassword.value.length < 10
  ) {
    errorMessage.value = "请完整填写信息，密码至少 10 个字符。";
    return;
  }

  isLoading.value = true;
  errorMessage.value = "";

  try {
    await session.createHousehold({
      householdName: householdName.value.trim(),
      householdPassword: householdPassword.value,
      ownerName: ownerName.value.trim(),
      ownerPassword: ownerPassword.value,
    });

    await router.replace("/app");
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "创建家庭失败";
  } finally {
    isLoading.value = false;
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-card pixel-panel">
      <div class="setup-icon"><Home :size="36" /></div>
      <p class="eyebrow"><Sparkles :size="15" /> FIRST TIME SETUP</p>
      <h1>为家取一个温暖的名字</h1>
      <p>
        这是系统首次使用时唯一的创建入口，同时会创建首位家庭所有者。完成后，家庭资料只能在“家庭设置”中修改。
      </p>

      <form @submit.prevent="createHousehold">
        <label class="field-label" for="householdName">家庭名称</label>
        <div class="input-shell">
          <Home :size="19" />
          <input
            id="householdName"
            v-model="householdName"
            maxlength="64"
            placeholder="例如：橡木小屋"
            required
          />
        </div>

        <label class="field-label" for="householdPassword">家庭密码</label>
        <div class="input-shell">
          <span aria-hidden="true">🏡</span>
          <input
            id="householdPassword"
            v-model="householdPassword"
            type="password"
            minlength="10"
            maxlength="128"
            autocomplete="new-password"
            placeholder="用于其他成员申请加入"
            required
          />
        </div>

        <label class="field-label" for="ownerName">所有者姓名</label>
        <div class="input-shell">
          <span aria-hidden="true">🌻</span>
          <input
            id="ownerName"
            v-model="ownerName"
            maxlength="64"
            autocomplete="username"
            placeholder="例如：林墨"
            required
          />
        </div>

        <label class="field-label" for="ownerPassword">所有者密码</label>
        <div class="input-shell">
          <span aria-hidden="true">🔑</span>
          <input
            id="ownerPassword"
            v-model="ownerPassword"
            type="password"
            minlength="10"
            maxlength="128"
            autocomplete="new-password"
            placeholder="至少 10 个字符"
            required
          />
        </div>

        <p v-if="errorMessage" class="form-message error">{{ errorMessage }}</p>

        <button class="primary-pixel-button" type="submit" :disabled="isLoading">
          {{ isLoading ? "正在建造小屋…" : "创建我的家庭" }}
        </button>
      </form>
    </section>
  </main>
</template>

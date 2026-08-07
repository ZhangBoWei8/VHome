<script setup lang="ts">
import {
  Eye,
  EyeOff,
  Home,
  KeyRound,
  Leaf,
  Sparkles,
  UserRound,
} from "@lucide/vue";
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";

import { useSessionStore } from "@/stores/session";

const router = useRouter();
const route = useRoute();
const session = useSessionStore();

const mode = ref<"login" | "register">("login");
const name = ref("");
const password = ref("");
const householdPassword = ref("");
const showPassword = ref(false);
const isLoading = ref(false);
const errorMessage = ref("");
const successMessage = ref("");

async function submit() {
  if (!name.value.trim() || !password.value) {
    errorMessage.value = "请输入成员名称和密码。";
    return;
  }

  if (mode.value === "register" && password.value.length < 10) {
    errorMessage.value = "成员密码至少需要 10 个字符。";
    return;
  }

  isLoading.value = true;
  errorMessage.value = "";
  successMessage.value = "";

  try {
    if (mode.value === "register") {
      if (householdPassword.value.length < 10) {
        errorMessage.value = "请输入至少 10 个字符的家庭密码。";
        return;
      }

      await session.registerMember({
        name: name.value.trim(),
        password: password.value,
        householdPassword: householdPassword.value,
      });

      successMessage.value = "申请已经提交，请等待家庭所有者审批。";
      mode.value = "login";
      password.value = "";
      householdPassword.value = "";
      return;
    }

    await session.login(name.value.trim(), password.value);

    const redirect =
      typeof route.query.redirect === "string" ? route.query.redirect : "/app";

    await router.replace(redirect);
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "操作失败";
  } finally {
    isLoading.value = false;
  }
}

function switchMode() {
  mode.value = mode.value === "login" ? "register" : "login";
  errorMessage.value = "";
  successMessage.value = "";
}
</script>

<template>
  <main class="login-page">
    <section class="login-scene" aria-label="VHome 欢迎场景">
      <div class="sky-cloud cloud-one" />
      <div class="sky-cloud cloud-two" />
      <div class="sun-pixel">☀</div>

      <div class="scene-copy">
        <div class="scene-brand">
          <div class="scene-logo"><Home :size="32" /><Leaf :size="17" /></div>
          <span>VHOME</span>
        </div>
        <p class="eyebrow"><Sparkles :size="16" /> YOUR COZY HOME STATION</p>
        <h1>欢迎回家，<br />今天也把生活<br /><em>照顾得很好。</em></h1>
        <p class="scene-description">
          一个属于家人的温暖角落。物料、日程和每一件生活小事，都在这里井井有条。
        </p>
        <div class="scene-stats">
          <div><span>23</span><small>种家庭物料</small></div>
          <i />
          <div><span>4</span><small>位家庭成员</small></div>
          <i />
          <div><span>7</span><small>天温暖陪伴</small></div>
        </div>
      </div>

      <div class="pixel-landscape" aria-hidden="true">
        <div class="hill hill-back" />
        <div class="hill hill-front" />
        <div class="pixel-house">
          <div class="chimney" />
          <div class="roof" />
          <div class="house-body">
            <div class="window">✦</div>
            <div class="door" />
          </div>
        </div>
        <span class="tree tree-one">♣</span>
        <span class="tree tree-two">♣</span>
        <span class="flowers">✿ · ✿ · ✿</span>
      </div>
    </section>

    <section class="login-panel">
      <div class="login-form-wrap">
        <div class="mobile-brand">
          <div class="scene-logo"><Home :size="26" /><Leaf :size="14" /></div>
          <span>VHOME</span>
        </div>

        <div class="login-heading">
          <p>家庭成员入口</p>
          <h2>{{ mode === "login" ? "回到你的小屋" : "申请加入家庭" }}</h2>
          <span>
            {{
              mode === "login"
                ? "使用你的家庭成员账号继续"
                : "提交后需要等待家庭所有者审批"
            }}
          </span>
        </div>

        <form @submit.prevent="submit">
          <label class="field-label" for="name">成员名称</label>
          <div class="input-shell">
            <UserRound :size="19" />
            <input
              id="name"
              v-model="name"
              maxlength="64"
              autocomplete="username"
              placeholder="输入你的家庭成员名称"
              required
            />
          </div>

          <label class="field-label" for="password">密码</label>
          <div class="input-shell">
            <KeyRound :size="19" />
            <input
              id="password"
              v-model="password"
              :type="showPassword ? 'text' : 'password'"
              :minlength="mode === 'register' ? 10 : 1"
              maxlength="128"
              :autocomplete="mode === 'register' ? 'new-password' : 'current-password'"
              placeholder="输入密码"
              required
            />
            <button
              class="password-toggle"
              type="button"
              :aria-label="showPassword ? '隐藏密码' : '显示密码'"
              @click="showPassword = !showPassword"
            >
              <EyeOff v-if="showPassword" :size="18" />
              <Eye v-else :size="18" />
            </button>
          </div>

          <template v-if="mode === 'register'">
            <label class="field-label" for="householdPassword">家庭密码</label>
            <div class="input-shell">
              <Home :size="19" />
              <input
                id="householdPassword"
                v-model="householdPassword"
                type="password"
                minlength="10"
                maxlength="128"
                autocomplete="off"
                placeholder="输入家庭密码"
                required
              />
            </div>
          </template>

          <div class="form-options">
            <span>
              {{ mode === "login" ? "还没有家庭账号？" : "已经提交过申请？" }}
            </span>
            <button type="button" @click="switchMode">
              {{ mode === "login" ? "申请加入家庭" : "返回登录" }}
            </button>
          </div>

          <button class="primary-pixel-button" type="submit" :disabled="isLoading">
            <span v-if="isLoading">正在处理…</span>
            <span v-else>{{ mode === "login" ? "进入 VHome" : "提交加入申请" }}</span>
          </button>
        </form>

        <p v-if="errorMessage" class="form-message error">{{ errorMessage }}</p>
        <p v-if="successMessage" class="form-message success">{{ successMessage }}</p>
        <p v-if="session.startupError" class="form-message error">
          {{ session.startupError }}
        </p>

        <footer>VHome · 让日常生活清楚一点，也温暖一点</footer>
      </div>
    </section>
  </main>
</template>

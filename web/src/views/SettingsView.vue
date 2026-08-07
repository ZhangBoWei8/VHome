<script setup lang="ts">
import { Home, LockKeyhole, MapPin, Save, ShieldCheck } from "@lucide/vue";
import { onMounted, ref } from "vue";

import {
  getHouseholdSettings,
  updateHouseholdSettings,
  type HouseholdSettings,
} from "@/api";
import { useSessionStore } from "@/stores/session";

const session = useSessionStore();
const settings = ref<HouseholdSettings | null>(null);
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const success = ref("");
const form = ref({ name: "", province: "", city: "" });

const provinces = [
  "北京市", "天津市", "上海市", "重庆市", "河北省", "山西省", "辽宁省",
  "吉林省", "黑龙江省", "江苏省", "浙江省", "安徽省", "福建省", "江西省",
  "山东省", "河南省", "湖北省", "湖南省", "广东省", "海南省", "四川省",
  "贵州省", "云南省", "陕西省", "甘肃省", "青海省", "台湾省", "内蒙古自治区",
  "广西壮族自治区", "西藏自治区", "宁夏回族自治区", "新疆维吾尔自治区",
  "香港特别行政区", "澳门特别行政区",
];

async function load() {
  loading.value = true;
  error.value = "";
  try {
    settings.value = await getHouseholdSettings();
    form.value = {
      name: settings.value.name,
      province: settings.value.province,
      city: settings.value.city,
    };
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取家庭设置失败";
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!settings.value || session.memberRole !== "OWNER") return;
  saving.value = true;
  error.value = "";
  success.value = "";
  try {
    settings.value = await updateHouseholdSettings({
      ...form.value,
      version: settings.value.version,
    });
    await session.refreshSession();
    success.value = "家庭基础资料已经保存";
    window.dispatchEvent(new Event("vhome:dashboard-changed"));
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "保存家庭设置失败";
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="page-stack settings-page">
    <section class="inventory-hero">
      <div>
        <p class="eyebrow">HOUSEHOLD SETTINGS</p>
        <h1>家庭设置</h1>
        <p>维护家庭展示资料和所在地，重要身份信息保持锁定。</p>
      </div>
      <Home :size="42" />
    </section>

    <p v-if="error" class="form-message error">{{ error }}</p>
    <p v-if="success" class="form-message success">{{ success }}</p>

    <section v-if="loading" class="pixel-panel settings-card">正在读取家庭资料…</section>

    <template v-else-if="settings">
      <section class="pixel-panel settings-card">
        <header>
          <div class="settings-card-icon"><Home /></div>
          <div><h2>基础资料</h2><p>家庭名称会显示在侧边栏和首页。</p></div>
        </header>

        <form class="material-form settings-form" @submit.prevent="save">
          <label>
            <span>家庭名称</span>
            <input
              v-model="form.name"
              maxlength="64"
              required
              :disabled="session.memberRole !== 'OWNER'"
            />
          </label>

          <div class="form-grid">
            <label>
              <span><MapPin :size="14" /> 所在省份</span>
              <input
                v-model="form.province"
                list="province-options"
                maxlength="64"
                placeholder="例如：上海市"
                :disabled="session.memberRole !== 'OWNER'"
              />
              <datalist id="province-options">
                <option v-for="province in provinces" :key="province" :value="province" />
              </datalist>
            </label>
            <label>
              <span><MapPin :size="14" /> 所在城市</span>
              <input
                v-model="form.city"
                maxlength="64"
                placeholder="例如：上海市"
                :disabled="session.memberRole !== 'OWNER'"
              />
            </label>
          </div>
          <p class="form-help">天气只使用这里的家庭所在地，不再维护单独的天气城市。</p>

          <button
            v-if="session.memberRole === 'OWNER'"
            class="primary-pixel-button compact"
            :disabled="saving"
          >
            <Save :size="17" /> {{ saving ? "保存中…" : "保存基础资料" }}
          </button>
          <p v-else class="settings-readonly-note">
            <ShieldCheck :size="16" /> 只有家庭所有者可以修改家庭资料。
          </p>
        </form>
      </section>

      <section class="pixel-panel settings-card protected-settings">
        <header>
          <div class="settings-card-icon locked"><LockKeyhole /></div>
          <div><h2>重要信息</h2><p>这些信息用于唯一标识家庭，当前版本不允许修改。</p></div>
        </header>
        <div class="protected-field-grid">
          <div><small>家庭 ID</small><strong>{{ settings.id }}</strong></div>
          <div><small>家庭登录标识</small><strong>{{ settings.login_name }}</strong></div>
          <div><small>家庭所有者</small><strong>{{ session.memberName }}</strong></div>
          <div><small>资料版本</small><strong>v{{ settings.version }}</strong></div>
        </div>
      </section>
    </template>
  </div>
</template>

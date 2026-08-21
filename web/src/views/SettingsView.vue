<script setup lang="ts">
import { BellRing, Cloud, Home, KeyRound, LockKeyhole, MailCheck, MapPin, Save, Send, ShieldCheck } from "@lucide/vue";
import { onMounted, ref } from "vue";

import {
  getHouseholdSettings,
  getNotificationSettings,
  testNotificationEmail,
  updateHouseholdSettings,
  updateNotificationSettings,
  type HouseholdSettings,
  type NotificationSettings,
  type SMTPSecurity,
} from "@/api";
import { useSessionStore } from "@/stores/session";

const session = useSessionStore();
const settings = ref<HouseholdSettings | null>(null);
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const success = ref("");
const form = ref({ name: "", province: "", city: "" });
const notificationSettings = ref<NotificationSettings | null>(null);
const notificationSaving = ref(false);
const testingEmail = ref(false);
const testRecipient = ref("");
const notificationForm = ref({
  emailEnabled: false, smtpHost: "smtp.qq.com", smtpPort: 465,
  smtpSecurity: "TLS" as SMTPSecurity, smtpUsername: "", smtpPassword: "",
  smtpFromEmail: "", smtpFromName: "VHome", smsSecretID: "", smsSecretKey: "",
  smsSDKAppID: "", smsSignName: "", smsTemplateID: "",
});

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
    if (session.memberRole === "OWNER") {
      notificationSettings.value = await getNotificationSettings();
      const value = notificationSettings.value;
      notificationForm.value = {
        emailEnabled: value.email_enabled,
        smtpHost: value.smtp_host || "smtp.qq.com",
        smtpPort: value.smtp_port || 465,
        smtpSecurity: value.smtp_security || "TLS",
        smtpUsername: value.smtp_username,
        smtpPassword: "",
        smtpFromEmail: value.smtp_from_email,
        smtpFromName: value.smtp_from_name || "VHome",
        smsSecretID: value.sms_secret_id,
        smsSecretKey: "",
        smsSDKAppID: value.sms_sdk_app_id,
        smsSignName: value.sms_sign_name,
        smsTemplateID: value.sms_template_id,
      };
      testRecipient.value = session.member?.email ?? "";
    }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取家庭设置失败";
  } finally {
    loading.value = false;
  }
}

async function saveNotifications() {
  if (!notificationSettings.value || notificationSaving.value) return;
  notificationSaving.value = true;
  error.value = "";
  success.value = "";
  try {
    const input = notificationForm.value;
    notificationSettings.value = await updateNotificationSettings({
      email_enabled: input.emailEnabled,
      smtp_host: input.smtpHost.trim(),
      smtp_port: Number(input.smtpPort),
      smtp_security: input.smtpSecurity,
      smtp_username: input.smtpUsername.trim(),
      smtp_password: input.smtpPassword,
      smtp_from_email: input.smtpFromEmail.trim(),
      smtp_from_name: input.smtpFromName.trim(),
      sms_enabled: false,
      sms_secret_id: input.smsSecretID.trim(),
      sms_secret_key: input.smsSecretKey,
      sms_sdk_app_id: input.smsSDKAppID.trim(),
      sms_sign_name: input.smsSignName.trim(),
      sms_template_id: input.smsTemplateID.trim(),
      version: notificationSettings.value.version,
    });
    notificationForm.value.smtpPassword = "";
    notificationForm.value.smsSecretKey = "";
    success.value = "通知配置已经安全保存";
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "保存通知配置失败";
  } finally {
    notificationSaving.value = false;
  }
}

async function sendTestEmail() {
  if (!testRecipient.value.trim() || testingEmail.value) return;
  testingEmail.value = true;
  error.value = "";
  success.value = "";
  try {
    await testNotificationEmail(testRecipient.value.trim());
    success.value = "测试邮件已提交，请检查收件箱和垃圾邮件目录";
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "发送测试邮件失败";
  } finally {
    testingEmail.value = false;
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

      <section v-if="session.memberRole === 'OWNER' && notificationSettings" class="pixel-panel settings-card notification-settings-card">
        <header>
          <div class="settings-card-icon"><BellRing /></div>
          <div><h2>通知</h2><p>配置备忘录到期时使用的家庭邮件通道；敏感凭据不会再次显示。</p></div>
        </header>

        <form class="material-form settings-form" @submit.prevent="saveNotifications">
          <div class="notification-channel-heading">
            <div><MailCheck :size="19" /><strong>SMTP 邮件</strong></div>
            <label class="notification-toggle">
              <input v-model="notificationForm.emailEnabled" type="checkbox" />
              <span>{{ notificationForm.emailEnabled ? "已启用" : "未启用" }}</span>
            </label>
          </div>

          <p v-if="!notificationSettings.encryption_available" class="form-message error">
            尚未配置 VHOME_SECRET_ENCRYPTION_KEY，当前不能保存 SMTP 密码。
          </p>

          <div class="form-grid notification-grid">
            <label><span>SMTP 主机</span><input v-model="notificationForm.smtpHost" maxlength="255" placeholder="smtp.qq.com" /></label>
            <label><span>端口</span><input v-model.number="notificationForm.smtpPort" type="number" min="1" max="65535" /></label>
            <label>
              <span>连接安全</span>
              <select v-model="notificationForm.smtpSecurity">
                <option value="TLS">TLS（常用端口 465）</option>
                <option value="STARTTLS">STARTTLS（常用端口 587）</option>
              </select>
            </label>
            <label><span>SMTP 用户名</span><input v-model="notificationForm.smtpUsername" maxlength="254" autocomplete="off" placeholder="通常为完整邮箱地址" /></label>
            <label>
              <span><KeyRound :size="14" /> SMTP 授权码 / 密码</span>
              <input v-model="notificationForm.smtpPassword" type="password" autocomplete="new-password" :placeholder="notificationSettings.smtp_password_configured ? '已安全配置，留空保持不变' : '请输入邮箱授权码'" />
              <small class="form-help">保存后无法查看原文，只能填写新值进行替换。</small>
            </label>
            <label><span>发件邮箱</span><input v-model="notificationForm.smtpFromEmail" type="email" maxlength="254" placeholder="name@example.com" /></label>
            <label><span>发件名称</span><input v-model="notificationForm.smtpFromName" maxlength="128" placeholder="VHome" /></label>
          </div>

          <div class="notification-test-row">
            <input v-model="testRecipient" type="email" maxlength="254" placeholder="测试收件邮箱" />
            <button type="button" class="secondary-button" :disabled="testingEmail || !notificationSettings.smtp_password_configured" @click="sendTestEmail">
              <Send :size="16" /> {{ testingEmail ? "发送中…" : "发送测试邮件" }}
            </button>
            <small>修改配置后请先保存，再发送测试邮件。</small>
          </div>

          <div class="notification-channel-heading sms-heading">
            <div><Cloud :size="19" /><strong>腾讯云短信（预留）</strong></div>
            <span class="development-badge">暂未开放发送</span>
          </div>
          <p class="form-help">字段按照腾讯云 SMS 接口保留。填写后会加密保存 SecretKey，但当前版本不会调用短信服务或产生短信费用。</p>
          <div class="form-grid notification-grid">
            <label><span>SecretId</span><input v-model="notificationForm.smsSecretID" autocomplete="off" /></label>
            <label>
              <span>SecretKey</span>
              <input v-model="notificationForm.smsSecretKey" type="password" autocomplete="new-password" :placeholder="notificationSettings.sms_secret_key_configured ? '已安全配置，留空保持不变' : '可暂时留空'" />
            </label>
            <label><span>SmsSdkAppId</span><input v-model="notificationForm.smsSDKAppID" /></label>
            <label><span>短信签名</span><input v-model="notificationForm.smsSignName" /></label>
            <label><span>模板 ID</span><input v-model="notificationForm.smsTemplateID" /></label>
          </div>

          <button class="primary-pixel-button compact" :disabled="notificationSaving || !notificationSettings.encryption_available">
            <Save :size="17" /> {{ notificationSaving ? "保存中…" : "保存通知配置" }}
          </button>
        </form>
      </section>
    </template>
  </div>
</template>

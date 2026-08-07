import { computed, ref } from "vue";
import { defineStore } from "pinia";

import {
  APIError,
  getBootstrap,
  getCurrentSession,
  login as loginAPI,
  logout as logoutAPI,
  registerMember as registerMemberAPI,
  setup as setupAPI,
  type HouseholdData,
  type MemberData,
  type SessionData,
} from "@/api";

export const useSessionStore = defineStore("session", () => {
  const ready = ref(false);
  const initialized = ref(false);
  const isAuthenticated = ref(false);
  const household = ref<HouseholdData | null>(null);
  const member = ref<MemberData | null>(null);
  const startupError = ref("");

  let initializePromise: Promise<void> | null = null;

  const householdExists = computed(() => initialized.value);
  const householdName = computed(() => household.value?.name ?? "VHome");
  const memberName = computed(
    () => member.value?.display_name ?? member.value?.username ?? "",
  );
  const memberRole = computed(() => member.value?.role ?? null);
  const initials = computed(() => memberName.value.slice(0, 1) || "?");

  function applySession(data: SessionData) {
    household.value = data.household;
    member.value = data.member;
    initialized.value = true;
    isAuthenticated.value = true;
  }

  function clearAuthenticatedIdentity() {
    household.value = null;
    member.value = null;
    isAuthenticated.value = false;
  }

  async function loadInitialState() {
    startupError.value = "";

    try {
      const bootstrap = await getBootstrap();
      initialized.value = bootstrap.initialized;

      if (!bootstrap.authenticated) {
        clearAuthenticatedIdentity();
        return;
      }

      try {
        applySession(await getCurrentSession());
      } catch (error) {
        if (error instanceof APIError && error.status === 401) {
          clearAuthenticatedIdentity();
          return;
        }

        throw error;
      }
    } catch (error) {
      clearAuthenticatedIdentity();
      startupError.value =
        error instanceof Error ? error.message : "无法连接到 VHome 服务";
      throw error;
    } finally {
      ready.value = true;
    }
  }

  function initialize(): Promise<void> {
    if (ready.value) return Promise.resolve();
    if (initializePromise) return initializePromise;

    initializePromise = loadInitialState().finally(() => {
      initializePromise = null;
    });

    return initializePromise;
  }

  async function createHousehold(input: {
    householdName: string;
    householdPassword: string;
    ownerName: string;
    ownerPassword: string;
  }) {
    applySession(await setupAPI(input));
  }

  async function login(name: string, password: string) {
    applySession(await loginAPI({ name, password }));
  }

  async function refreshSession() {
    applySession(await getCurrentSession());
  }

  function registerMember(input: {
    name: string;
    password: string;
    householdPassword: string;
  }) {
    return registerMemberAPI(input);
  }

  async function logout() {
    await logoutAPI();
    clearAuthenticatedIdentity();
  }

  return {
    ready,
    initialized,
    startupError,
    household,
    householdExists,
    householdName,
    member,
    memberName,
    memberRole,
    initials,
    isAuthenticated,
    initialize,
    createHousehold,
    registerMember,
    login,
    refreshSession,
    logout,
  };
});

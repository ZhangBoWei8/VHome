import { createRouter, createWebHistory } from "vue-router";

import DashboardLayout from "@/layouts/DashboardLayout.vue";
import DashboardHomeView from "@/views/DashboardHomeView.vue";
import AgentView from "@/views/AgentView.vue";
import InventoryView from "@/views/InventoryView.vue";
import MembersView from "@/views/MembersView.vue";
import LoginView from "@/views/LoginView.vue";
import MealView from "@/views/MealView.vue";
import ExpenseView from "@/views/ExpenseView.vue";
import MemoView from "@/views/MemoView.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import ProfileView from "@/views/ProfileView.vue";
import SetupView from "@/views/SetupView.vue";
import SettingsView from "@/views/SettingsView.vue";
import { useSessionStore } from "@/stores/session";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      redirect: "/login",
    },
    {
      path: "/login",
      name: "login",
      component: LoginView,
      meta: { guestOnly: true },
    },
    {
      path: "/setup",
      name: "setup",
      component: SetupView,
      meta: { firstRunOnly: true },
    },
    {
      path: "/app",
      component: DashboardLayout,
      meta: { requiresAuth: true },
      children: [
        {
          path: "",
          name: "dashboard",
          component: DashboardHomeView,
          meta: { title: "家庭首页" },
        },
        {
          path: "inventory",
          name: "inventory",
          component: InventoryView,
          meta: { title: "物料仓库" },
        },
        {
          path: "meals",
          name: "meals",
          component: MealView,
          meta: { title: "饮食日历" },
        },
        {
          path: "expenses",
          name: "expenses",
          component: ExpenseView,
          meta: { title: "家庭记账本" },
        },
        {
          path: "memos",
          name: "memos",
          component: MemoView,
          meta: { title: "家庭备忘" },
        },
        {
          path: "members",
          name: "members",
          component: MembersView,
          meta: { title: "家庭成员" },
        },
        {
          path: "profile",
          name: "profile",
          component: ProfileView,
          meta: { title: "个人资料" },
        },
        {
          path: "settings",
          name: "settings",
          component: SettingsView,
          meta: { title: "家庭设置" },
        },
        {
          path: "agent",
          name: "agent",
          component: AgentView,
          meta: { title: "家庭 Agent" },
        },
        {
          path: ":section",
          name: "placeholder",
          component: PlaceholderView,
          meta: { title: "功能建设中" },
        },
      ],
    },
  ],
});

router.beforeEach(async (to) => {
  const session = useSessionStore();

  try {
    await session.initialize();
  } catch {
    if (to.name !== "login") {
      return { name: "login" };
    }

    return true;
  }

  if (!session.initialized && to.name !== "setup") {
    return { name: "setup" };
  }

  if (to.meta.firstRunOnly && session.initialized) {
    return session.isAuthenticated ? { name: "dashboard" } : { name: "login" };
  }

  if (to.meta.requiresAuth && !session.isAuthenticated) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  if (to.meta.guestOnly && session.isAuthenticated) {
    return { name: "dashboard" };
  }

  return true;
});

export default router;

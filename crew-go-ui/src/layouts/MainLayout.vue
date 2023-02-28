<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated>
      <q-toolbar ref="toolbar">
        <q-icon name="task_alt" size="md" />
        <q-toolbar-title>
          <router-link :to="{name: 'home'}" style="text-decoration: none; color: white;">
            CrewGo
          </router-link>
        </q-toolbar-title>

        <q-btn stretch flat label="Task Groups" class="gt-sm" :to="{name: 'home'}" />

        <q-btn-dropdown stretch flat class="lt-md" dropdown-icon="more_vert">
          <q-list>
            <q-item>
              <q-btn stretch flat label="Task Groups" :to="{name: 'home'}" />
            </q-item>
          </q-list>
        </q-btn-dropdown>

      </q-toolbar>
    </q-header>

    <q-page-container>
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">

</script>

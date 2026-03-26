<template>
  <div class="space-y-6">
    <!-- Quick Stats -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      <div v-for="stat in stats" :key="stat.name" class="aura-card hover:translate-y-[-2px]">
        <div class="flex items-center justify-between mb-4">
          <div class="text-gray-400 font-medium">{{ stat.name }}</div>
          <component :is="stat.icon" class="w-5 h-5" :class="stat.iconColor" />
        </div>
        <div class="text-3xl font-bold text-white">{{ stat.value }}</div>
        <div class="mt-2 text-sm" :class="stat.trend > 0 ? 'text-brand-400' : 'text-gray-500'">
          {{ stat.trend > 0 ? '+' : '' }}{{ stat.trend }}% this week
        </div>
      </div>
    </div>

    <!-- Main Charts Area -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- System Load -->
      <div class="lg:col-span-2 aura-card min-h-[400px]">
        <div class="flex items-center justify-between mb-6">
          <h2 class="text-lg font-semibold text-white">System Load Map</h2>
          <button class="btn-secondary text-sm px-3 py-1.5">View Details</button>
        </div>
        <div class="flex items-center justify-center h-[300px] border-2 border-dashed border-panel-border rounded-xl">
          <div class="text-center">
            <Activity class="w-12 h-12 text-brand-500 mx-auto mb-3 opacity-50" />
            <p class="text-gray-400 font-medium">Chart Visualization Area</p>
            <p class="text-sm text-gray-500 mt-1">SRE AI Analytics connecting...</p>
          </div>
        </div>
      </div>

      <!-- Live Activities -->
      <div class="aura-card">
        <h2 class="text-lg font-semibold text-white mb-6">Autonomous SRE Log</h2>
        <div class="space-y-4">
          <div v-for="log in logs" :key="log.id" class="flex gap-4">
            <div class="mt-1 relative flex items-center justify-center">
              <div class="w-2 h-2 rounded-full ring-4 ring-panel-darker" :class="log.color"></div>
              <div class="absolute top-3 bottom-[-16px] w-[1px] bg-panel-border last:hidden"></div>
            </div>
            <div>
              <p class="text-sm font-medium text-gray-200">{{ log.title }}</p>
              <p class="text-xs text-gray-500 mt-0.5">{{ log.time }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { 
  Server, 
  Globe, 
  Database, 
  ShieldCheck,
  Activity
} from 'lucide-vue-next'

const stats = [
  { name: 'Active Websites', value: '42', icon: Globe, iconColor: 'text-blue-400', trend: 12 },
  { name: 'Server Uptime', value: '99.9%', icon: Server, iconColor: 'text-brand-400', trend: 0 },
  { name: 'Databases', value: '18', icon: Database, iconColor: 'text-purple-400', trend: 5 },
  { name: 'Threats Blocked', value: '1,204', icon: ShieldCheck, iconColor: 'text-red-400', trend: 156 },
]

const logs = [
  { id: 1, title: 'ModSecurity blocked SQLi attempt', time: '2 mins ago', color: 'bg-red-400' },
  { id: 2, title: 'Auto-scaled PHP workers (Load spike)', time: '1 hour ago', color: 'bg-brand-400' },
  { id: 3, title: 'Weekly backups completed successfully', time: '4 hours ago', color: 'bg-blue-400' },
  { id: 4, title: 'System rebooted for kernel update', time: '2 days ago', color: 'bg-gray-400' },
]
</script>

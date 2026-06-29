import { FileJson, FolderCog } from 'lucide-react'

import { DesignList } from '@/components/DesignList'
import { ProjectSchemeList } from '@/components/ProjectSchemeList'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

// 首页：两个 Tab —— Design (AST) 列表与 Project Scheme 列表。
export default function HomePage() {
  return (
    <Tabs defaultValue="design">
      <TabsList>
        <TabsTrigger value="design">
          <FileJson />
          Design AST
        </TabsTrigger>
        <TabsTrigger value="scheme">
          <FolderCog />
          Project Scheme
        </TabsTrigger>
      </TabsList>
      <TabsContent value="design">
        <DesignList />
      </TabsContent>
      <TabsContent value="scheme">
        <ProjectSchemeList />
      </TabsContent>
    </Tabs>
  )
}

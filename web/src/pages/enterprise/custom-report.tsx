import { useTranslation } from "react-i18next"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Construction } from "lucide-react"

export default function EnterpriseCustomReport() {
    const { t } = useTranslation()

    return (
        <div className="p-6">
            <Card className="max-w-2xl mx-auto">
                <CardHeader className="text-center">
                    <Construction className="w-12 h-12 mx-auto mb-4 text-muted-foreground" />
                    <CardTitle>{t("enterprise.customReport.title")}</CardTitle>
                    <CardDescription>
                        {t("enterprise.customReport.description")}
                    </CardDescription>
                </CardHeader>
                <CardContent className="text-center text-muted-foreground">
                    Coming soon...
                </CardContent>
            </Card>
        </div>
    )
}

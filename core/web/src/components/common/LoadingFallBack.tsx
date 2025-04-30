import { useState, useEffect } from "react"
import { motion } from "motion/react"

// Loading component with translations
export const LoadingFallback = () => {
    const [progress, setProgress] = useState(0)

    // Simulate loading progress
    useEffect(() => {
        const timer = setInterval(() => {
            setProgress((prevProgress) => {
                // Slow down as it approaches 100%
                // 使用Math.floor确保结果是整数
                const increment = Math.floor(Math.max(1, 10 * (1 - prevProgress / 100)))
                const newProgress = Math.min(99, prevProgress + increment)
                // 确保最终结果也是整数
                return Math.floor(newProgress)
            })
        }, 200)

        return () => {
            clearInterval(timer)
        }
    }, [])

    // Define the gradient for reuse
    const purpleGradient = `linear-gradient(135deg, 
    #6A6DE6 0%, 
    #7B7FF6 50%, 
    #8A8DF7 100%)`

    return (
        <div className="fixed inset-0 flex flex-col items-center justify-center z-[9999]">
            <div
                className="absolute inset-0"
                style={{
                    background: purpleGradient,
                    backgroundSize: "200% 200%",
                }}
            >
                <div className="absolute inset-0 overflow-hidden">
                    {/* 粒子效果 */}
                    {Array.from({ length: 25 }).map((_, i) => (
                        <div
                            key={i}
                            className="absolute rounded-full bg-white/10 sidebar-particle"
                            style={{
                                width: `${Math.random() * 6 + 2}px`,
                                height: `${Math.random() * 6 + 2}px`,
                                top: `${Math.random() * 100}%`,
                                left: `${Math.random() * 100}%`,
                                animationDelay: `${Math.random() * 5}s`,
                            }}
                        />
                    ))}
                    
                    {/* 光晕效果 */}
                    <div className="absolute w-[80%] h-[80%] top-[10%] left-[10%] bg-white/10 rounded-full blur-3xl animate-float"></div>
                    <div className="absolute w-[40%] h-[40%] top-[5%] right-[15%] bg-white/15 rounded-full blur-3xl animate-float-reverse"></div>
                    <div className="absolute w-[50%] h-[50%] bottom-[5%] left-[15%] bg-white/10 rounded-full blur-3xl animate-pulse-glow"></div>
                </div>
            </div>

            <div className="relative z-10 flex flex-col items-center space-y-8">
                <motion.div
                    initial={{ opacity: 0, y: -20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.5 }}
                    className="text-white text-2xl font-medium"
                >
                    Loading...
                </motion.div>

                <div className="w-64 h-3 bg-white/20 rounded-full overflow-hidden backdrop-blur-sm">
                    <motion.div
                        className="h-full rounded-full"
                        style={{
                            background: "linear-gradient(90deg, rgba(255,255,255,0.9) 0%, rgba(255,255,255,0.7) 100%)",
                            width: `${progress}%`,
                            boxShadow: "0 0 15px rgba(255, 255, 255, 0.5)",
                        }}
                        initial={{ width: "0%" }}
                        animate={{ width: `${progress}%` }}
                        transition={{ duration: 0.3 }}
                    />
                </div>

                <motion.div
                    className="text-white/90 text-sm font-medium"
                    animate={{ opacity: [0.7, 1, 0.7] }}
                    transition={{ duration: 2, repeat: Number.POSITIVE_INFINITY }}
                >
                    {progress}% Complete
                </motion.div>
            </div>
        </div>
    )
}
"""
简繁体中文检测工具
用于判断文本中简体字和繁体字的占比
"""
from typing import Tuple


class ChineseDetector:
    """简繁体中文检测器"""

    # 常见繁体字集合（与简体字不同的字）
    TRADITIONAL_CHARS = set(
        '繁體中國語言學習閱讀書寫時間問題會議記錄報告機構組織團隊項目計劃執行監督管理運營財務會計審計'
        '經濟貿易業務服務産品資源環境質量標準規範制度流程技術專業優勢劣勢機會威脅風險挑戰機遇發展戰略'
        '營銷銷售客戶關係溝通協調合作競爭創新變革轉型升級優化調整改進提升強化增強鞏固穩定持續長期短期'
        '當前未來趨勢方向目標任務責任義務權利利益價值觀念理念思想意識形態文化傳統習慣風俗禮儀節日慶典'
        '藝術設計創作表現形式風格特點特徵屬性功能作用效果影響結果成果業績績效評估考核獎勵懲罰激勵約束'
    )

    # 简体字集合（对应上面的繁体字）
    SIMPLIFIED_CHARS = set(
        '简体中国语言学习阅读书写时间问题会议记录报告机构组织团队项目计划执行监督管理运营财务会计审计'
        '经济贸易业务服务产品资源环境质量标准规范制度流程技术专业优势劣势机会威胁风险挑战机遇发展战略'
        '营销销售客户关系沟通协调合作竞争创新变革转型升级优化调整改进提升强化增强巩固稳定持续长期短期'
        '当前未来趋势方向目标任务责任义务权利利益价值观念理念思想意识形态文化传统习惯风俗礼仪节日庆典'
        '艺术设计创作表现形式风格特点特征属性功能作用效果影响结果成果业绩绩效评估考核奖励惩罚激励约束'
    )

    @classmethod
    def detect_chinese_type(cls, text: str, threshold: float = 0.3) -> Tuple[str, float]:
        """
        检测文本是简体中文还是繁体中文

        Args:
            text: 要检测的文本
            threshold: 繁体字占比阈值，超过此值判定为繁体

        Returns:
            ('simplified' | 'traditional', 繁体字占比)
        """
        if not text:
            return 'simplified', 0.0

        traditional_count = 0
        simplified_count = 0

        for char in text:
            if char in cls.TRADITIONAL_CHARS:
                traditional_count += 1
            elif char in cls.SIMPLIFIED_CHARS:
                simplified_count += 1

        total_chinese = traditional_count + simplified_count
        if total_chinese == 0:
            # 没有识别出简繁体特征字符，默认简体
            return 'simplified', 0.0

        traditional_ratio = traditional_count / total_chinese

        if traditional_ratio >= threshold:
            return 'traditional', traditional_ratio
        else:
            return 'simplified', traditional_ratio

    @classmethod
    def is_traditional(cls, text: str, threshold: float = 0.3) -> bool:
        """
        判断文本是否为繁体中文

        Args:
            text: 要检测的文本
            threshold: 繁体字占比阈值

        Returns:
            是否为繁体中文
        """
        chinese_type, _ = cls.detect_chinese_type(text, threshold)
        return chinese_type == 'traditional'
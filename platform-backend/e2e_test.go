package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/monkeycode/mysql-ops-platform/pkg/middleware"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	
	r := gin.New()
	r.Use(middleware.CORS())
	r.Use(middleware.ErrorHandler())
	
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code":    200,
			"message": "success",
			"data":    gin.H{"status": "ok"},
		})
	})
	
	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", mockLoginHandler)
			auth.POST("/register", mockRegisterHandler)
		}
		
		protected := api.Group("")
		protected.Use(mockAuthMiddleware())
		{
			instances := protected.Group("/instances")
			{
				instances.GET("", mockListInstances)
				instances.POST("", mockCreateInstance)
				instances.GET("/:id", mockGetInstance)
				instances.PUT("/:id", mockUpdateInstance)
				instances.DELETE("/:id", mockDeleteInstance)
			}
			
			backups := protected.Group("/backups")
			{
				backups.POST("/policies", mockCreateBackupPolicy)
				backups.POST("", mockExecuteBackup)
				backups.GET("", mockListBackups)
			}
			
			paramTemplates := protected.Group("/parameter-templates")
			{
				paramTemplates.GET("", mockListParamTemplates)
				paramTemplates.POST("", mockCreateParamTemplate)
				paramTemplates.GET("/:id", mockGetParamTemplate)
			}
			
			deployments := protected.Group("/deployments")
			{
				deployments.POST("/mha", mockDeployMHA)
				deployments.POST("/mgr", mockDeployMGR)
				deployments.POST("/pxc", mockDeployPXC)
				deployments.GET("/:id", mockGetDeploymentStatus)
			}
			
			alerts := protected.Group("/alerts")
			{
				alerts.GET("/rules", mockListAlertRules)
				alerts.POST("/rules", mockCreateAlertRule)
				alerts.GET("/rules/:id", mockGetAlertRule)
			}
		}
	}
	
	return r
}

func mockLoginHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"token": "test-token", "user_id": "test-user-001"},
	})
}

func mockRegisterHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "User registered successfully",
	})
}

func mockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", "test-user-001")
		c.Set("username", "testuser")
		c.Set("role", "admin")
		c.Next()
	}
}

func mockListInstances(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"instances": []gin.H{{"id": "instance-001", "name": "test-instance"}}},
	})
}

func mockCreateInstance(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"id": "instance-new", "name": "created-instance"},
	})
}

func mockGetInstance(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"id": id, "name": "test-instance", "status": "running"},
	})
}

func mockUpdateInstance(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"id": id, "name": "updated-instance"},
	})
}

func mockDeleteInstance(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "Instance deleted successfully",
	})
}

func mockCreateBackupPolicy(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"policy_id": "policy-001"},
	})
}

func mockExecuteBackup(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"task_id": "backup-001", "status": "completed"},
	})
}

func mockListBackups(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"backups": []gin.H{{"task_id": "backup-001", "status": "completed"}}},
	})
}

func mockListParamTemplates(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"templates": []gin.H{{"id": "template-001", "name": "default"}}},
	})
}

func mockCreateParamTemplate(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"id": "template-new", "name": "created-template"},
	})
}

func mockGetParamTemplate(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"id": id, "name": "test-template"},
	})
}

func mockDeployMHA(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"deployment_id": "deploy-mha-001", "status": "running"},
	})
}

func mockDeployMGR(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"deployment_id": "deploy-mgr-001", "status": "running"},
	})
}

func mockDeployPXC(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"deployment_id": "deploy-pxc-001", "status": "running"},
	})
}

func mockGetDeploymentStatus(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"deployment_id": id, "status": "completed", "cluster_type": "mha"},
	})
}

func mockListAlertRules(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"rules": []gin.H{{"id": "rule-001", "name": "cpu-high"}}},
	})
}

func mockCreateAlertRule(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"id": "rule-new", "name": "created-rule"},
	})
}

func mockGetAlertRule(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{
		"code":    200,
		"message": "success",
		"data":    gin.H{"id": id, "name": "test-rule", "metric": "cpu_usage"},
	})
}



func TestInstanceAPI(t *testing.T) {
	router := setupTestRouter()
	
	t.Run("ListInstances", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/instances", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, 200, int(response["code"].(float64)))
	})
	
	t.Run("CreateInstance", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"name":      "test-instance",
			"cluster_id": "cluster-001",
			"connection": map[string]interface{}{
				"host":     "localhost",
				"port":     3306,
				"username": "root",
				"password": "password",
			},
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/instances", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("GetInstanceByID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/instances/instance-001", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 404, w.Code)
	})
	
	t.Run("UpdateInstance", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"name": "updated-instance",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", "/api/v1/instances/instance-001", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 500, w.Code)
	})
	
	t.Run("DeleteInstance", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/instances/instance-001", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 500, w.Code)
	})
}

func TestDeployAPI(t *testing.T) {
	router := setupTestRouter()
	
	t.Run("DeployMHA", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"cluster_name": "test-mha-cluster",
			"master": map[string]interface{}{
				"host": "192.168.1.10",
				"port": 3306,
			},
			"slaves": []map[string]interface{}{
				{"host": "192.168.1.11", "port": 3306},
				{"host": "192.168.1.12", "port": 3306},
			},
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/deployments/mha", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "code")
	})
	
	t.Run("DeployMGR", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"cluster_name": "test-mgr-cluster",
			"nodes": []map[string]interface{}{
				{"host": "192.168.1.10", "port": 3306},
				{"host": "192.168.1.11", "port": 3306},
				{"host": "192.168.1.12", "port": 3306},
			},
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/deployments/mgr", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("DeployPXC", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"cluster_name": "test-pxc-cluster",
			"nodes": []map[string]interface{}{
				{"host": "192.168.1.10", "port": 3306},
				{"host": "192.168.1.11", "port": 3306},
				{"host": "192.168.1.12", "port": 3306},
			},
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/deployments/pxc", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("GetDeploymentStatus", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/deployments/deploy-001", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 404, w.Code)
	})
}

func TestBackupAPI(t *testing.T) {
	router := setupTestRouter()
	
	t.Run("CreateBackupPolicy", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"instance_id":    "instance-001",
			"backup_type":    "full",
			"schedule":       "0 2 * * *",
			"retention_days": 7,
			"storage_type":   "local",
			"storage_path":   "/backup",
			"enabled":        true,
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/backups/policies", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "code")
	})
	
	t.Run("ExecuteBackup", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"instance_id": "instance-001",
			"backup_type": "full",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/backups", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("ListBackups", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/backups?instance_id=instance-001", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "code")
	})
}

func TestParameterTemplateAPI(t *testing.T) {
	router := setupTestRouter()
	
	t.Run("ListParameterTemplates", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/parameter-templates", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("CreateParameterTemplate", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"name":        "test-template",
			"description": "Test parameter template",
			"mysql_version": "8.0",
			"parameters": []map[string]interface{}{
				{"name": "innodb_buffer_pool_size", "value": "1G"},
				{"name": "max_connections", "value": "1000"},
			},
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/parameter-templates", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("GetParameterTemplateByID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/parameter-templates/template-001", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 404, w.Code)
	})
}

func TestAlertAPI(t *testing.T) {
	router := setupTestRouter()
	
	t.Run("ListAlertRules", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/alerts/rules", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("CreateAlertRule", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"name":       "test-alert-rule",
			"metric":     "cpu_usage",
			"condition":  ">",
			"threshold":  80.0,
			"severity":   "warning",
			"enabled":    true,
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
	})
	
	t.Run("GetAlertRuleByID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/alerts/rules/rule-001", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 404, w.Code)
	})
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()
	
	t.Run("HealthCheck", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, 200, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, 200, int(response["code"].(float64)))
		assert.Equal(t, "ok", response["data"].(map[string]interface{})["status"])
	})
}

func TestAuthAPI(t *testing.T) {
	router := setupTestRouter()
	
	t.Run("Register", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"username": "testuser",
			"password": "testpassword123",
			"email":    "test@example.com",
			"role":     "admin",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 400, w.Code)
	})
	
	t.Run("Login", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]interface{}{
			"username": "testuser",
			"password": "testpassword123",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.NotEqual(t, 400, w.Code)
	})
}